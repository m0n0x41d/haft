package projecttypeenvselectionauthority

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
)

const sourceAdapterPolicyDomain = "haft.project-typeenv.head-selection-source-adapter-policy/v1"

// ProjectTypeEnvHeadSelectionSourceAdapterPolicy pins one concrete verified
// source adapter admitted by a resolver-policy edition. For example, today's
// controlling-terminal adapter can be pinned here without making TTY capture
// part of the semantic TypeEnv authority contract.
type ProjectTypeEnvHeadSelectionSourceAdapterPolicy struct {
	method         authority.SpeechActMethodDescription
	executedWithin authority.SystemRef
	contextPolicy  authority.SpeechActContextPolicy
	digest         authority.Digest
	canonicalJSON  []byte
}

type sourceAdapterPolicyProjection struct {
	Schema                  string `json:"schema"`
	MethodRef               string `json:"method_ref"`
	MethodDescriptionRef    string `json:"method_description_ref"`
	MethodDescriptionDigest string `json:"method_description_digest"`
	ExecutedWithin          string `json:"executed_within_system_ref"`
	ContextPolicyRef        string `json:"context_policy_ref"`
	ContextPolicyDigest     string `json:"context_policy_digest"`
}

func SealProjectTypeEnvHeadSelectionSourceAdapterPolicy(
	method authority.SpeechActMethodDescription,
	executedWithin authority.SystemRef,
	contextPolicy authority.SpeechActContextPolicy,
) (ProjectTypeEnvHeadSelectionSourceAdapterPolicy, error) {
	methodRef, methodRefOK := method.MethodRef()
	descriptionRef, descriptionRefOK := method.Ref()
	descriptionDigest, descriptionDigestOK := method.Digest()
	contextPolicyRef, contextPolicyRefOK := contextPolicy.Ref()
	contextPolicyDigest, contextPolicyDigestOK := contextPolicy.Digest()
	canonicalSystem, err := authority.NewSystemRef(executedWithin.String())
	valid := methodRefOK && descriptionRefOK && descriptionDigestOK && err == nil &&
		contextPolicyRefOK && contextPolicyDigestOK && canonicalSystem == executedWithin
	if !valid {
		return ProjectTypeEnvHeadSelectionSourceAdapterPolicy{}, fmt.Errorf(
			"TypeEnv source-adapter policy coordinates are invalid",
		)
	}
	projection := sourceAdapterPolicyProjection{
		Schema:                  "haft.project-typeenv.head-selection-source-adapter-policy/v1",
		MethodRef:               methodRef.String(),
		MethodDescriptionRef:    descriptionRef.String(),
		MethodDescriptionDigest: descriptionDigest.String(),
		ExecutedWithin:          executedWithin.String(),
		ContextPolicyRef:        contextPolicyRef.String(),
		ContextPolicyDigest:     contextPolicyDigest.String(),
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSourceAdapterPolicy{}, err
	}
	digest, err := digestCanonical(sourceAdapterPolicyDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSourceAdapterPolicy{}, err
	}
	return ProjectTypeEnvHeadSelectionSourceAdapterPolicy{
		method:         method,
		executedWithin: executedWithin,
		contextPolicy:  contextPolicy,
		digest:         digest,
		canonicalJSON:  canonical,
	}, nil
}

func (policy ProjectTypeEnvHeadSelectionSourceAdapterPolicy) VerifySource(
	source authority.VerifiedSpeechActSourceV2,
) error {
	method, methodOK := source.MethodDescription()
	executedWithin, executedWithinOK := source.ExecutedWithin()
	contextPolicy, contextPolicyOK := source.ContextPolicy()
	if !methodOK || !executedWithinOK || !contextPolicyOK {
		return fmt.Errorf("TypeEnv source adapter coordinates are unavailable")
	}
	if !sameSpeechActMethodDescription(method, policy.method) ||
		executedWithin != policy.executedWithin ||
		!sameSpeechActContextPolicy(contextPolicy, policy.contextPolicy) {
		return fmt.Errorf("SpeechAct source is not admitted by exact source-adapter policy")
	}
	return nil
}

func DecodeProjectTypeEnvHeadSelectionSourceAdapterPolicy(
	method authority.SpeechActMethodDescription,
	executedWithin authority.SystemRef,
	contextPolicy authority.SpeechActContextPolicy,
	canonical []byte,
	digest authority.Digest,
) (ProjectTypeEnvHeadSelectionSourceAdapterPolicy, error) {
	if len(canonical) == 0 || len(canonical) > 64*1024 {
		return ProjectTypeEnvHeadSelectionSourceAdapterPolicy{}, fmt.Errorf(
			"TypeEnv source-adapter policy has invalid canonical size",
		)
	}
	projection := sourceAdapterPolicyProjection{}
	if err := decodeStrictJSON(canonical, &projection); err != nil {
		return ProjectTypeEnvHeadSelectionSourceAdapterPolicy{}, err
	}
	rebuilt, err := SealProjectTypeEnvHeadSelectionSourceAdapterPolicy(
		method,
		executedWithin,
		contextPolicy,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSourceAdapterPolicy{}, err
	}
	if rebuilt.digest != digest || !bytes.Equal(rebuilt.canonicalJSON, canonical) {
		return ProjectTypeEnvHeadSelectionSourceAdapterPolicy{}, fmt.Errorf(
			"TypeEnv source-adapter policy is not exact canonical material",
		)
	}
	return rebuilt, nil
}

func (policy ProjectTypeEnvHeadSelectionSourceAdapterPolicy) ExactAgainst(
	method authority.SpeechActMethodDescription,
	executedWithin authority.SystemRef,
	contextPolicy authority.SpeechActContextPolicy,
) error {
	rebuilt, err := SealProjectTypeEnvHeadSelectionSourceAdapterPolicy(
		method,
		executedWithin,
		contextPolicy,
	)
	if err != nil {
		return err
	}
	if rebuilt.digest != policy.digest || !bytes.Equal(rebuilt.canonicalJSON, policy.canonicalJSON) {
		return fmt.Errorf("TypeEnv source-adapter policy differs from exact adapter")
	}
	return nil
}

func sameSpeechActContextPolicy(
	left authority.SpeechActContextPolicy,
	right authority.SpeechActContextPolicy,
) bool {
	leftRef, leftRefOK := left.Ref()
	rightRef, rightRefOK := right.Ref()
	leftDigest, leftDigestOK := left.Digest()
	rightDigest, rightDigestOK := right.Digest()
	return leftRefOK && rightRefOK && leftDigestOK && rightDigestOK &&
		leftRef == rightRef && leftDigest == rightDigest
}

func sameSpeechActMethodDescription(
	left authority.SpeechActMethodDescription,
	right authority.SpeechActMethodDescription,
) bool {
	leftMethod, leftMethodOK := left.MethodRef()
	rightMethod, rightMethodOK := right.MethodRef()
	leftDescription, leftDescriptionOK := left.Ref()
	rightDescription, rightDescriptionOK := right.Ref()
	leftDigest, leftDigestOK := left.Digest()
	rightDigest, rightDigestOK := right.Digest()
	present := leftMethodOK && rightMethodOK && leftDescriptionOK &&
		rightDescriptionOK && leftDigestOK && rightDigestOK
	return present && leftMethod == rightMethod &&
		leftDescription == rightDescription && leftDigest == rightDigest
}

func (policy ProjectTypeEnvHeadSelectionSourceAdapterPolicy) MethodDescription() authority.SpeechActMethodDescription {
	return policy.method
}

func (policy ProjectTypeEnvHeadSelectionSourceAdapterPolicy) ExecutedWithin() authority.SystemRef {
	return policy.executedWithin
}

func (policy ProjectTypeEnvHeadSelectionSourceAdapterPolicy) ContextPolicy() authority.SpeechActContextPolicy {
	return policy.contextPolicy
}

func (policy ProjectTypeEnvHeadSelectionSourceAdapterPolicy) Digest() authority.Digest {
	return policy.digest
}

func (policy ProjectTypeEnvHeadSelectionSourceAdapterPolicy) CanonicalJSON() []byte {
	return append([]byte(nil), policy.canonicalJSON...)
}
