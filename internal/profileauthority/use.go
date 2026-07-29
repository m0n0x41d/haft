package profileauthority

import (
	"fmt"
	"slices"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
)

const authorityUseDigestDomain = "haft.profile-authority.authority-use/v2\x00"

// AuthorityUseRecord is the immutable historical fact that one exact sealed
// profile-declaration authority use and one admission request were consumed by
// one committed admission result. It is not authority, a permission, Work, an
// admission, or a reusable capability.
//
// It has no wire constructor. A new record requires an exact sealed
// AdmittedUse. A persistence adapter reconstructs historical material by
// reconstructing the resolution, replay-evaluating it at the stored
// consumed_at, and recomputing this record before exact byte/digest comparison.
type AuthorityUseRecord struct {
	state *authorityUseState
}

type authorityUseState struct {
	ref                      ProfileDeclarationAuthorityUseRef
	admittedUse              AdmittedUse
	admissionRequestDigest   authority.Digest
	committedAdmissionRef    CommittedProfileAdmissionRef
	committedAdmissionDigest authority.Digest
	consumedAt               time.Time
	digest                   authority.Digest
	canonical                []byte
}

type authorityUseSnapshot struct {
	ref                      ProfileDeclarationAuthorityUseRef
	projectRoot              authority.ProjectRoot
	actionKind               authority.ActionKind
	projectBindingDigest     authority.Digest
	resolutionRef            ProfileDeclarationAuthorityResolutionRef
	resolutionDigest         authority.Digest
	basisRef                 BasisRef
	basisDigest              authority.Digest
	permissionRef            authority.PermissionRef
	permissionDigest         authority.Digest
	contentRef               authority.AuthorizationContentRef
	contentDigest            authority.Digest
	singleUseKey             authority.SingleUseKey
	admissionRequestDigest   authority.Digest
	committedAdmissionRef    CommittedProfileAdmissionRef
	committedAdmissionDigest authority.Digest
	consumedAt               time.Time
	digest                   authority.Digest
	canonical                []byte
}

type authorityUseJSONV2 struct {
	Schema                     string `json:"schema"`
	UseRef                     string `json:"use_ref"`
	ProjectRoot                string `json:"project_root"`
	ActionKind                 string `json:"action_kind"`
	ProjectBindingDigest       string `json:"project_binding_digest"`
	AuthorityResolutionRef     string `json:"authority_resolution_ref"`
	AuthorityResolutionDigest  string `json:"authority_resolution_digest"`
	AuthorityBasisRef          string `json:"authority_basis_ref"`
	AuthorityBasisDigest       string `json:"authority_basis_digest"`
	PermissionRef              string `json:"permission_ref"`
	PermissionDigest           string `json:"permission_digest"`
	AuthorizationContentRef    string `json:"authorization_content_ref"`
	AuthorizationContentDigest string `json:"authorization_content_digest"`
	SingleUseKey               string `json:"single_use_key"`
	AdmissionRequestDigest     string `json:"admission_request_digest"`
	CommittedAdmissionRef      string `json:"committed_admission_ref"`
	CommittedAdmissionDigest   string `json:"committed_admission_digest"`
	ConsumedAt                 string `json:"consumed_at"`
}

func canonicalAuthorityUse(
	ref ProfileDeclarationAuthorityUseRef,
	use AdmittedUse,
	admissionRequestDigest authority.Digest,
	committedAdmissionRef CommittedProfileAdmissionRef,
	committedAdmissionDigest authority.Digest,
	consumedAt time.Time,
) (authorityUseSnapshot, error) {
	if !ref.valid() {
		return authorityUseSnapshot{}, fmt.Errorf(
			"profile authority use ref is invalid",
		)
	}
	if !use.valid() {
		return authorityUseSnapshot{}, fmt.Errorf(
			"profile authority use requires a sealed admitted use",
		)
	}
	if !validDigest(admissionRequestDigest) {
		return authorityUseSnapshot{}, fmt.Errorf(
			"profile authority use admission-request digest is invalid",
		)
	}
	if !committedAdmissionRef.valid() || !validDigest(committedAdmissionDigest) {
		return authorityUseSnapshot{}, fmt.Errorf(
			"profile authority use committed admission pair is invalid",
		)
	}
	canonicalConsumedAt := canonicalTime(consumedAt)
	judgedAt, _ := use.JudgedAt()
	if canonicalConsumedAt.IsZero() || !canonicalConsumedAt.Equal(judgedAt) {
		return authorityUseSnapshot{}, fmt.Errorf(
			"profile authority use consumed_at must equal the gate judgement time",
		)
	}
	projectRoot, actionKind, projectBindingDigest, _ := use.ProjectBinding()
	resolutionRef, resolutionDigest, _ := use.Resolution()
	basisRef, basisDigest, _ := use.Basis()
	permissionRef, permissionDigest, _ := use.Permission()
	contentRef, contentDigest, _ := use.AuthorizationContent()
	singleUseKey, _ := use.SingleUseKey()
	dto := authorityUseJSONV2{
		Schema:                     "haft.profile-authority.authority-use/v2",
		UseRef:                     ref.String(),
		ProjectRoot:                projectRoot.String(),
		ActionKind:                 actionKind.String(),
		ProjectBindingDigest:       projectBindingDigest.String(),
		AuthorityResolutionRef:     resolutionRef.String(),
		AuthorityResolutionDigest:  resolutionDigest.String(),
		AuthorityBasisRef:          basisRef.String(),
		AuthorityBasisDigest:       basisDigest.String(),
		PermissionRef:              permissionRef.String(),
		PermissionDigest:           permissionDigest.String(),
		AuthorizationContentRef:    contentRef.String(),
		AuthorizationContentDigest: contentDigest.String(),
		SingleUseKey:               singleUseKey.String(),
		AdmissionRequestDigest:     admissionRequestDigest.String(),
		CommittedAdmissionRef:      committedAdmissionRef.String(),
		CommittedAdmissionDigest:   committedAdmissionDigest.String(),
		ConsumedAt:                 formatTime(canonicalConsumedAt),
	}
	digest, canonical, err := canonicalDigest(authorityUseDigestDomain, dto)
	if err != nil {
		return authorityUseSnapshot{}, err
	}
	return authorityUseSnapshot{
		ref:                      ref,
		projectRoot:              projectRoot,
		actionKind:               actionKind,
		projectBindingDigest:     projectBindingDigest,
		resolutionRef:            resolutionRef,
		resolutionDigest:         resolutionDigest,
		basisRef:                 basisRef,
		basisDigest:              basisDigest,
		permissionRef:            permissionRef,
		permissionDigest:         permissionDigest,
		contentRef:               contentRef,
		contentDigest:            contentDigest,
		singleUseKey:             singleUseKey,
		admissionRequestDigest:   admissionRequestDigest,
		committedAdmissionRef:    committedAdmissionRef,
		committedAdmissionDigest: committedAdmissionDigest,
		consumedAt:               canonicalConsumedAt,
		digest:                   digest,
		canonical:                canonical,
	}, nil
}

func newAuthorityUseRecord(
	ref ProfileDeclarationAuthorityUseRef,
	use AdmittedUse,
	admissionRequestDigest authority.Digest,
	committedAdmissionRef CommittedProfileAdmissionRef,
	committedAdmissionDigest authority.Digest,
	consumedAt time.Time,
) (AuthorityUseRecord, error) {
	snapshot, err := canonicalAuthorityUse(
		ref,
		use,
		admissionRequestDigest,
		committedAdmissionRef,
		committedAdmissionDigest,
		consumedAt,
	)
	if err != nil {
		return AuthorityUseRecord{}, err
	}
	state := authorityUseState{
		ref:                      ref,
		admittedUse:              use,
		admissionRequestDigest:   admissionRequestDigest,
		committedAdmissionRef:    committedAdmissionRef,
		committedAdmissionDigest: committedAdmissionDigest,
		consumedAt:               snapshot.consumedAt,
		digest:                   snapshot.digest,
		canonical:                slices.Clone(snapshot.canonical),
	}
	return AuthorityUseRecord{state: &state}, nil
}

func (record AuthorityUseRecord) snapshot() (authorityUseSnapshot, bool) {
	if record.state == nil {
		return authorityUseSnapshot{}, false
	}
	snapshot, err := canonicalAuthorityUse(
		record.state.ref,
		record.state.admittedUse,
		record.state.admissionRequestDigest,
		record.state.committedAdmissionRef,
		record.state.committedAdmissionDigest,
		record.state.consumedAt,
	)
	if err != nil {
		return authorityUseSnapshot{}, false
	}
	matches := snapshot.digest.String() == record.state.digest.String()
	matches = matches && slices.Equal(snapshot.canonical, record.state.canonical)
	return snapshot, matches
}

func (record AuthorityUseRecord) Ref() (
	ProfileDeclarationAuthorityUseRef,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.ref, ok
}

func (record AuthorityUseRecord) Digest() (authority.Digest, bool) {
	snapshot, ok := record.snapshot()
	return snapshot.digest, ok
}

func (record AuthorityUseRecord) CanonicalBytes() ([]byte, bool) {
	snapshot, ok := record.snapshot()
	if !ok {
		return nil, false
	}
	return slices.Clone(snapshot.canonical), true
}

func (record AuthorityUseRecord) ProjectBinding() (
	authority.ProjectRoot,
	authority.ActionKind,
	authority.Digest,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.projectRoot,
		snapshot.actionKind,
		snapshot.projectBindingDigest,
		ok
}

func (record AuthorityUseRecord) Resolution() (
	ProfileDeclarationAuthorityResolutionRef,
	authority.Digest,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.resolutionRef, snapshot.resolutionDigest, ok
}

func (record AuthorityUseRecord) Basis() (BasisRef, authority.Digest, bool) {
	snapshot, ok := record.snapshot()
	return snapshot.basisRef, snapshot.basisDigest, ok
}

func (record AuthorityUseRecord) Permission() (
	authority.PermissionRef,
	authority.Digest,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.permissionRef, snapshot.permissionDigest, ok
}

func (record AuthorityUseRecord) AuthorizationContent() (
	authority.AuthorizationContentRef,
	authority.Digest,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.contentRef, snapshot.contentDigest, ok
}

func (record AuthorityUseRecord) SingleUseKey() (
	authority.SingleUseKey,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.singleUseKey, ok
}

func (record AuthorityUseRecord) AdmissionRequestDigest() (
	authority.Digest,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.admissionRequestDigest, ok
}

func (record AuthorityUseRecord) CommittedAdmission() (
	CommittedProfileAdmissionRef,
	authority.Digest,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.committedAdmissionRef, snapshot.committedAdmissionDigest, ok
}

func (record AuthorityUseRecord) ConsumedAt() (time.Time, bool) {
	snapshot, ok := record.snapshot()
	return snapshot.consumedAt, ok
}
