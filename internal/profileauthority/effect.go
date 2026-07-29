package profileauthority

import (
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/authority"
)

const institutedEffectDigestDomain = "haft.profile-authority.instituted-permission-effect/v2\x00"

type institutedEffectState struct {
	permission       Permission
	projectRoot      authority.ProjectRoot
	speechActRef     authority.SpeechActRef
	speechActDigest  authority.Digest
	permissionRef    authority.PermissionRef
	permissionDigest authority.Digest
	digest           authority.Digest
	canonical        []byte
}

// InstitutedEffect records the domain consequence separately from both the
// communicative Work and the resulting U.Commitment(MAY).
type InstitutedEffect struct {
	state *institutedEffectState
}

type institutedEffectJSONV2 struct {
	Schema           string `json:"schema"`
	ProjectRoot      string `json:"project_root"`
	SpeechActRef     string `json:"speech_act_ref"`
	SpeechActDigest  string `json:"speech_act_digest"`
	PermissionRef    string `json:"permission_ref"`
	PermissionDigest string `json:"permission_digest"`
}

func NewInstitutedEffect(permission Permission) (InstitutedEffect, error) {
	state, err := canonicalInstitutedEffect(permission)
	if err != nil {
		return InstitutedEffect{}, err
	}
	return InstitutedEffect{state: &state}, nil
}

func canonicalInstitutedEffect(permission Permission) (institutedEffectState, error) {
	if !permission.valid() {
		return institutedEffectState{}, fmt.Errorf(
			"instituted profile effect requires an exact permission",
		)
	}
	permissionRef, _ := permission.Ref()
	permissionDigest, _ := permission.Digest()
	speechActRef, speechActDigest, _ := permission.SourceSpeechAct()
	dto := institutedEffectJSONV2{
		Schema:           "haft.profile-authority.instituted-permission-effect/v2",
		ProjectRoot:      permission.state.projectRoot.String(),
		SpeechActRef:     speechActRef.String(),
		SpeechActDigest:  speechActDigest.String(),
		PermissionRef:    permissionRef.String(),
		PermissionDigest: permissionDigest.String(),
	}
	digest, canonical, err := canonicalDigest(institutedEffectDigestDomain, dto)
	if err != nil {
		return institutedEffectState{}, err
	}
	return institutedEffectState{
		permission:       permission,
		projectRoot:      permission.state.projectRoot,
		speechActRef:     speechActRef,
		speechActDigest:  speechActDigest,
		permissionRef:    permissionRef,
		permissionDigest: permissionDigest,
		digest:           digest,
		canonical:        canonical,
	}, nil
}

func (effect InstitutedEffect) valid() bool {
	if effect.state == nil {
		return false
	}
	rebuilt, err := canonicalInstitutedEffect(effect.state.permission)
	if err != nil {
		return false
	}
	return rebuilt.digest.String() == effect.state.digest.String() &&
		slices.Equal(rebuilt.canonical, effect.state.canonical)
}

func (effect InstitutedEffect) Digest() (authority.Digest, bool) {
	if !effect.valid() {
		return authority.Digest{}, false
	}
	return effect.state.digest, true
}

func (effect InstitutedEffect) CanonicalBytes() ([]byte, bool) {
	if !effect.valid() {
		return nil, false
	}
	return slices.Clone(effect.state.canonical), true
}
