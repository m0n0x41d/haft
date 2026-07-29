package profileauthority

import (
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/authority"
)

const fourRefBasisDigestDomain = "haft.profile-authority.four-ref-basis/v1\x00"

type fourRefBasisState struct {
	ref                 BasisRef
	projectRoot         authority.ProjectRoot
	speechActRef        authority.SpeechActRef
	speechActDigest     authority.Digest
	contentRef          authority.AuthorizationContentRef
	contentDigest       authority.Digest
	permissionRef       authority.PermissionRef
	permissionDigest    authority.Digest
	contextPolicyRef    authority.ContextPolicyRef
	contextPolicyDigest authority.Digest
	digest              authority.Digest
	canonical           []byte
}

// FourRefBasis is exactly the source-native authority basis: SpeechAct,
// authorization content, resulting MAY permission, and context policy. It has
// no receipt, presentation, resolution, or legacy-projection dependency.
type FourRefBasis struct {
	state *fourRefBasisState
}

type fourRefBasisJSONV1 struct {
	Schema              string `json:"schema"`
	BasisRef            string `json:"basis_ref"`
	ProjectRoot         string `json:"project_root"`
	SpeechActRef        string `json:"speech_act_ref"`
	SpeechActDigest     string `json:"speech_act_digest"`
	ContentRef          string `json:"authorization_content_ref"`
	ContentDigest       string `json:"authorization_content_digest"`
	PermissionRef       string `json:"permission_ref"`
	PermissionDigest    string `json:"permission_digest"`
	ContextPolicyRef    string `json:"context_policy_ref"`
	ContextPolicyDigest string `json:"context_policy_digest"`
}

func NewFourRefBasis(
	prepared PreparedAuthorization,
	permission Permission,
) (FourRefBasis, error) {
	state, err := canonicalFourRefBasis(prepared, permission)
	if err != nil {
		return FourRefBasis{}, err
	}
	return FourRefBasis{state: &state}, nil
}

func canonicalFourRefBasis(
	prepared PreparedAuthorization,
	permission Permission,
) (fourRefBasisState, error) {
	if !prepared.valid() || !permission.valid() {
		return fourRefBasisState{}, fmt.Errorf(
			"four-ref basis requires exact prepared authorization and permission",
		)
	}
	if permission.state.prepared.state.digest.String() != prepared.state.digest.String() {
		return fourRefBasisState{}, fmt.Errorf(
			"permission belongs to another prepared profile authorization",
		)
	}
	contentRef, _ := prepared.state.content.Ref()
	contentDigest, _ := prepared.state.content.Digest()
	permissionRef, _ := permission.Ref()
	permissionDigest, _ := permission.Digest()
	speechActRef, speechActDigest, _ := permission.SourceSpeechAct()
	policyRef, _ := prepared.state.policy.Ref()
	policyDigest, _ := prepared.state.policy.Digest()
	root, _ := prepared.state.content.ProjectRoot()
	dto := fourRefBasisJSONV1{
		Schema:              "haft.profile-authority.four-ref-basis/v1",
		BasisRef:            prepared.state.basisRef.String(),
		ProjectRoot:         root.String(),
		SpeechActRef:        speechActRef.String(),
		SpeechActDigest:     speechActDigest.String(),
		ContentRef:          contentRef.String(),
		ContentDigest:       contentDigest.String(),
		PermissionRef:       permissionRef.String(),
		PermissionDigest:    permissionDigest.String(),
		ContextPolicyRef:    policyRef.String(),
		ContextPolicyDigest: policyDigest.String(),
	}
	digest, canonical, err := canonicalDigest(fourRefBasisDigestDomain, dto)
	if err != nil {
		return fourRefBasisState{}, err
	}
	return fourRefBasisState{
		ref:                 prepared.state.basisRef,
		projectRoot:         root,
		speechActRef:        speechActRef,
		speechActDigest:     speechActDigest,
		contentRef:          contentRef,
		contentDigest:       contentDigest,
		permissionRef:       permissionRef,
		permissionDigest:    permissionDigest,
		contextPolicyRef:    policyRef,
		contextPolicyDigest: policyDigest,
		digest:              digest,
		canonical:           canonical,
	}, nil
}

func (basis FourRefBasis) valid() bool {
	if basis.state == nil {
		return false
	}
	dto := fourRefBasisJSONV1{
		Schema:              "haft.profile-authority.four-ref-basis/v1",
		BasisRef:            basis.state.ref.String(),
		ProjectRoot:         basis.state.projectRoot.String(),
		SpeechActRef:        basis.state.speechActRef.String(),
		SpeechActDigest:     basis.state.speechActDigest.String(),
		ContentRef:          basis.state.contentRef.String(),
		ContentDigest:       basis.state.contentDigest.String(),
		PermissionRef:       basis.state.permissionRef.String(),
		PermissionDigest:    basis.state.permissionDigest.String(),
		ContextPolicyRef:    basis.state.contextPolicyRef.String(),
		ContextPolicyDigest: basis.state.contextPolicyDigest.String(),
	}
	digest, canonical, err := canonicalDigest(fourRefBasisDigestDomain, dto)
	if err != nil {
		return false
	}
	return digest.String() == basis.state.digest.String() &&
		slices.Equal(canonical, basis.state.canonical)
}

func (basis FourRefBasis) Ref() (BasisRef, bool) {
	if !basis.valid() {
		return BasisRef{}, false
	}
	return basis.state.ref, true
}

func (basis FourRefBasis) Digest() (authority.Digest, bool) {
	if !basis.valid() {
		return authority.Digest{}, false
	}
	return basis.state.digest, true
}

func (basis FourRefBasis) CanonicalBytes() ([]byte, bool) {
	if !basis.valid() {
		return nil, false
	}
	return slices.Clone(basis.state.canonical), true
}

func (basis FourRefBasis) SpeechAct() (
	authority.SpeechActRef,
	authority.Digest,
	bool,
) {
	if !basis.valid() {
		return authority.SpeechActRef{}, authority.Digest{}, false
	}
	return basis.state.speechActRef, basis.state.speechActDigest, true
}

func (basis FourRefBasis) AuthorizationContent() (
	authority.AuthorizationContentRef,
	authority.Digest,
	bool,
) {
	if !basis.valid() {
		return authority.AuthorizationContentRef{}, authority.Digest{}, false
	}
	return basis.state.contentRef, basis.state.contentDigest, true
}

func (basis FourRefBasis) Permission() (
	authority.PermissionRef,
	authority.Digest,
	bool,
) {
	if !basis.valid() {
		return authority.PermissionRef{}, authority.Digest{}, false
	}
	return basis.state.permissionRef, basis.state.permissionDigest, true
}

func (basis FourRefBasis) ContextPolicy() (
	authority.ContextPolicyRef,
	authority.Digest,
	bool,
) {
	if !basis.valid() {
		return authority.ContextPolicyRef{}, authority.Digest{}, false
	}
	return basis.state.contextPolicyRef, basis.state.contextPolicyDigest, true
}
