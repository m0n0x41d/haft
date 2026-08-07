package projecttypeenvselectionauthority

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/m0n0x41d/haft/internal/authority"
)

const (
	resolverPolicySchema       = "haft.project-typeenv.head-selection-authority-resolver-policy/v1"
	resolverPolicyDomain       = "haft.project-typeenv.head-selection-authority-resolver-policy/v1"
	maximumResolverPolicyBytes = 256 * 1024
)

type ProjectTypeEnvHeadSelectionResolverPolicyRef struct{ value string }

func NewProjectTypeEnvHeadSelectionResolverPolicyRef(
	raw string,
) (ProjectTypeEnvHeadSelectionResolverPolicyRef, error) {
	if !validPolicyCoordinate(raw) {
		return ProjectTypeEnvHeadSelectionResolverPolicyRef{}, fmt.Errorf(
			"TypeEnv resolver-policy ref is not canonical",
		)
	}
	return ProjectTypeEnvHeadSelectionResolverPolicyRef{value: raw}, nil
}

func (ref ProjectTypeEnvHeadSelectionResolverPolicyRef) String() string { return ref.value }

type ProjectTypeEnvHeadSelectionResolverPolicyEdition struct{ value string }

func NewProjectTypeEnvHeadSelectionResolverPolicyEdition(
	raw string,
) (ProjectTypeEnvHeadSelectionResolverPolicyEdition, error) {
	if !validPolicyCoordinate(raw) {
		return ProjectTypeEnvHeadSelectionResolverPolicyEdition{}, fmt.Errorf(
			"TypeEnv resolver-policy edition is not canonical",
		)
	}
	return ProjectTypeEnvHeadSelectionResolverPolicyEdition{value: raw}, nil
}

func (edition ProjectTypeEnvHeadSelectionResolverPolicyEdition) String() string {
	return edition.value
}

// ProjectTypeEnvHeadSelectionResolverPolicy is a reusable project/context
// policy artifact. It names the exact communicative source contract, explicit
// ProjectID-to-ProjectRoot relation, one admitted action, and its effective
// window. It does not embed one SpeechAct, request, C, or Stage; that
// occurrence-specific tuple belongs to AuthorityResolutionBasis. Genesis and
// Transition require distinct policies because one exact ContextPolicy scopes
// exactly one action.
type ProjectTypeEnvHeadSelectionResolverPolicy struct {
	ref             ProjectTypeEnvHeadSelectionResolverPolicyRef
	edition         ProjectTypeEnvHeadSelectionResolverPolicyEdition
	digest          authority.Digest
	effectiveWindow authority.TimeWindow
	action          ProjectTypeEnvHeadSelectionAction
	sourceContract  ProjectTypeEnvHeadSelectionSpeechActSourceContract
	sourceAdapter   ProjectTypeEnvHeadSelectionSourceAdapterPolicy
	projectBinding  ProjectAuthorityContextBinding
	canonicalJSON   []byte
}

type ProjectTypeEnvHeadSelectionResolverPolicyInput struct {
	Ref             ProjectTypeEnvHeadSelectionResolverPolicyRef
	Edition         ProjectTypeEnvHeadSelectionResolverPolicyEdition
	EffectiveWindow authority.TimeWindow
	Action          ProjectTypeEnvHeadSelectionAction
	SourceContract  ProjectTypeEnvHeadSelectionSpeechActSourceContract
	SourceAdapter   ProjectTypeEnvHeadSelectionSourceAdapterPolicy
	ProjectBinding  ProjectAuthorityContextBinding
}

type resolverPolicyProjection struct {
	Schema               string `json:"schema"`
	Ref                  string `json:"ref"`
	Edition              string `json:"edition"`
	EffectiveFrom        string `json:"effective_from"`
	EffectiveUntil       string `json:"effective_until"`
	Project              string `json:"project_id"`
	ProjectRoot          string `json:"project_root"`
	ProjectBindingDigest string `json:"project_context_binding_digest"`
	JudgementContext     string `json:"judgement_context_ref"`
	SourceContractDigest string `json:"speech_act_source_contract_digest"`
	SourceAdapterDigest  string `json:"speech_act_source_adapter_policy_digest"`
	AdmittedAction       string `json:"admitted_action"`
	CurrentnessBoundary  string `json:"currentness_boundary"`
}

func SealProjectTypeEnvHeadSelectionResolverPolicy(
	input ProjectTypeEnvHeadSelectionResolverPolicyInput,
) (ProjectTypeEnvHeadSelectionResolverPolicy, error) {
	if !validPolicyCoordinate(input.Ref.String()) ||
		!validPolicyCoordinate(input.Edition.String()) {
		return ProjectTypeEnvHeadSelectionResolverPolicy{}, fmt.Errorf(
			"TypeEnv resolver policy identity is incomplete",
		)
	}
	effective, err := authority.NewTimeWindow(
		input.EffectiveWindow.From(),
		input.EffectiveWindow.Until(),
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionResolverPolicy{}, err
	}
	if err := input.SourceContract.ExactAgainst(input.ProjectBinding.Context()); err != nil {
		return ProjectTypeEnvHeadSelectionResolverPolicy{}, err
	}
	if _, err := input.Action.AuthorityActionKind(); err != nil {
		return ProjectTypeEnvHeadSelectionResolverPolicy{}, err
	}
	if !input.ProjectBinding.ExactFor(
		input.ProjectBinding.Project(),
		input.ProjectBinding.Root(),
		input.ProjectBinding.Context(),
	) {
		return ProjectTypeEnvHeadSelectionResolverPolicy{}, fmt.Errorf(
			"resolver policy project-context binding is invalid",
		)
	}
	if err := input.SourceAdapter.ExactAgainst(
		input.SourceAdapter.MethodDescription(),
		input.SourceAdapter.ExecutedWithin(),
		input.SourceAdapter.ContextPolicy(),
	); err != nil {
		return ProjectTypeEnvHeadSelectionResolverPolicy{}, err
	}
	if err := verifyResolverPolicySourceCompatibility(
		input.Action,
		input.SourceContract,
		input.SourceAdapter,
	); err != nil {
		return ProjectTypeEnvHeadSelectionResolverPolicy{}, err
	}
	policy := ProjectTypeEnvHeadSelectionResolverPolicy{
		ref:             input.Ref,
		edition:         input.Edition,
		effectiveWindow: effective,
		action:          input.Action,
		sourceContract:  input.SourceContract,
		sourceAdapter:   input.SourceAdapter,
		projectBinding:  input.ProjectBinding,
	}
	projection := projectResolverPolicy(policy)
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ProjectTypeEnvHeadSelectionResolverPolicy{}, err
	}
	if len(canonical) > maximumResolverPolicyBytes {
		return ProjectTypeEnvHeadSelectionResolverPolicy{}, fmt.Errorf(
			"TypeEnv resolver policy exceeds %d bytes",
			maximumResolverPolicyBytes,
		)
	}
	digest, err := digestCanonical(resolverPolicyDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionResolverPolicy{}, err
	}
	policy.digest = digest
	policy.canonicalJSON = canonical
	return policy, nil
}

func projectResolverPolicy(
	policy ProjectTypeEnvHeadSelectionResolverPolicy,
) resolverPolicyProjection {
	return resolverPolicyProjection{
		Schema:               resolverPolicySchema,
		Ref:                  policy.ref.String(),
		Edition:              policy.edition.String(),
		EffectiveFrom:        formatTime(policy.effectiveWindow.From()),
		EffectiveUntil:       formatTime(policy.effectiveWindow.Until()),
		Project:              policy.projectBinding.Project().String(),
		ProjectRoot:          policy.projectBinding.Root().String(),
		ProjectBindingDigest: policy.projectBinding.Digest().String(),
		JudgementContext:     policy.projectBinding.Context().String(),
		SourceContractDigest: policy.sourceContract.Digest().String(),
		SourceAdapterDigest:  policy.sourceAdapter.Digest().String(),
		AdmittedAction:       policy.action.String(),
		CurrentnessBoundary:  "pure_policy_does_not_prove_registry_currentness;_trusted_effect_must_reread_exact_policy_before_use",
	}
}

func verifyResolverPolicySourceCompatibility(
	action ProjectTypeEnvHeadSelectionAction,
	contract ProjectTypeEnvHeadSelectionSpeechActSourceContract,
	adapter ProjectTypeEnvHeadSelectionSourceAdapterPolicy,
) error {
	contextPolicy := adapter.ContextPolicy()
	context, contextOK := contextPolicy.BoundedContext()
	recognizedActType, actTypeOK := contextPolicy.RecognizedActType()
	institutedKind, institutedKindOK := contextPolicy.InstitutedObjectKind()
	modality, modalityOK := contextPolicy.InstitutionalModality()
	scopedAction, scopedActionOK := contextPolicy.ScopedAction()
	expectedAction, err := action.AuthorityActionKind()
	if err != nil {
		return err
	}
	present := contextOK && actTypeOK && institutedKindOK &&
		modalityOK && scopedActionOK
	if !present {
		return fmt.Errorf(
			"resolver policy source-adapter ContextPolicy coordinates are incomplete",
		)
	}
	if context != contract.Context() {
		return fmt.Errorf(
			"resolver policy source-adapter context differs from semantic source contract",
		)
	}
	if recognizedActType != contract.ActType() {
		return fmt.Errorf(
			"resolver policy source-adapter act type differs from semantic source contract",
		)
	}
	if institutedKind.String() != "U.Commitment" || modality.String() != "MAY" {
		return fmt.Errorf(
			"resolver policy source-adapter institutional effect differs from semantic source contract",
		)
	}
	if scopedAction != expectedAction {
		return fmt.Errorf(
			"resolver policy source-adapter scoped action differs from admitted action",
		)
	}
	return nil
}

func (policy ProjectTypeEnvHeadSelectionResolverPolicy) ExactAgainst(
	contract ProjectTypeEnvHeadSelectionSpeechActSourceContract,
	adapter ProjectTypeEnvHeadSelectionSourceAdapterPolicy,
	binding ProjectAuthorityContextBinding,
) error {
	rebuilt, err := SealProjectTypeEnvHeadSelectionResolverPolicy(
		ProjectTypeEnvHeadSelectionResolverPolicyInput{
			Ref:             policy.ref,
			Edition:         policy.edition,
			EffectiveWindow: policy.effectiveWindow,
			Action:          policy.action,
			SourceContract:  contract,
			SourceAdapter:   adapter,
			ProjectBinding:  binding,
		},
	)
	if err != nil {
		return err
	}
	if rebuilt.digest != policy.digest || !bytes.Equal(rebuilt.canonicalJSON, policy.canonicalJSON) {
		return fmt.Errorf("resolver policy does not match exact source contract/project binding")
	}
	return nil
}

func DecodeProjectTypeEnvHeadSelectionResolverPolicy(
	input ProjectTypeEnvHeadSelectionResolverPolicyInput,
	canonical []byte,
	digest authority.Digest,
) (ProjectTypeEnvHeadSelectionResolverPolicy, error) {
	if len(canonical) == 0 || len(canonical) > maximumResolverPolicyBytes {
		return ProjectTypeEnvHeadSelectionResolverPolicy{}, fmt.Errorf(
			"TypeEnv resolver policy has invalid canonical size",
		)
	}
	projection := resolverPolicyProjection{}
	if err := decodeStrictJSON(canonical, &projection); err != nil {
		return ProjectTypeEnvHeadSelectionResolverPolicy{}, err
	}
	rebuilt, err := SealProjectTypeEnvHeadSelectionResolverPolicy(input)
	if err != nil {
		return ProjectTypeEnvHeadSelectionResolverPolicy{}, err
	}
	if rebuilt.digest != digest || !bytes.Equal(rebuilt.canonicalJSON, canonical) {
		return ProjectTypeEnvHeadSelectionResolverPolicy{}, fmt.Errorf(
			"TypeEnv resolver policy is not exact canonical material",
		)
	}
	return rebuilt, nil
}

func (policy ProjectTypeEnvHeadSelectionResolverPolicy) Ref() ProjectTypeEnvHeadSelectionResolverPolicyRef {
	return policy.ref
}

func (policy ProjectTypeEnvHeadSelectionResolverPolicy) Edition() ProjectTypeEnvHeadSelectionResolverPolicyEdition {
	return policy.edition
}

func (policy ProjectTypeEnvHeadSelectionResolverPolicy) Digest() authority.Digest {
	return policy.digest
}

func (policy ProjectTypeEnvHeadSelectionResolverPolicy) EffectiveWindow() authority.TimeWindow {
	return policy.effectiveWindow
}

func (policy ProjectTypeEnvHeadSelectionResolverPolicy) Action() ProjectTypeEnvHeadSelectionAction {
	return policy.action
}

func (policy ProjectTypeEnvHeadSelectionResolverPolicy) SourceContract() ProjectTypeEnvHeadSelectionSpeechActSourceContract {
	return policy.sourceContract
}

func (policy ProjectTypeEnvHeadSelectionResolverPolicy) SourceAdapter() ProjectTypeEnvHeadSelectionSourceAdapterPolicy {
	return policy.sourceAdapter
}

func (policy ProjectTypeEnvHeadSelectionResolverPolicy) ProjectBinding() ProjectAuthorityContextBinding {
	return policy.projectBinding
}

func (policy ProjectTypeEnvHeadSelectionResolverPolicy) CanonicalJSON() []byte {
	return append([]byte(nil), policy.canonicalJSON...)
}

func validPolicyCoordinate(raw string) bool {
	return raw != "" && raw == strings.TrimSpace(raw) && len(raw) <= 1024 &&
		!strings.ContainsFunc(raw, unicode.IsControl)
}
