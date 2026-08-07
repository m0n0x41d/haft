package projecttypeenvselectionauthority

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/authority"
)

const (
	speechActRecordRefPrefix  = "project-typeenv-head-selection-speech-act-record:"
	authorityBasisRefPrefix   = "project-typeenv-head-selection-authority-resolution-basis:"
	authorityResolutionPrefix = "project-typeenv-head-selection-authority-resolution:"
)

type ProjectTypeEnvHeadSelectionSpeechActRecordRef struct {
	digest authority.Digest
}

type ProjectTypeEnvHeadSelectionAuthorityResolutionBasisRef struct {
	digest authority.Digest
}

func ParseProjectTypeEnvHeadSelectionAuthorityResolutionBasisRef(
	raw string,
) (ProjectTypeEnvHeadSelectionAuthorityResolutionBasisRef, error) {
	digest, err := parseDigestRef("authority resolution basis", authorityBasisRefPrefix, raw)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityResolutionBasisRef{}, err
	}
	return ProjectTypeEnvHeadSelectionAuthorityResolutionBasisRef{digest: digest}, nil
}

func (ref ProjectTypeEnvHeadSelectionAuthorityResolutionBasisRef) Digest() authority.Digest {
	return ref.digest
}

func (ref ProjectTypeEnvHeadSelectionAuthorityResolutionBasisRef) String() string {
	return authorityBasisRefPrefix + ref.digest.String()
}

func ParseProjectTypeEnvHeadSelectionSpeechActRecordRef(
	raw string,
) (ProjectTypeEnvHeadSelectionSpeechActRecordRef, error) {
	digest, err := parseDigestRef("SpeechAct-record", speechActRecordRefPrefix, raw)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecordRef{}, err
	}
	return ProjectTypeEnvHeadSelectionSpeechActRecordRef{digest: digest}, nil
}

func (ref ProjectTypeEnvHeadSelectionSpeechActRecordRef) Digest() authority.Digest {
	return ref.digest
}

func (ref ProjectTypeEnvHeadSelectionSpeechActRecordRef) String() string {
	return speechActRecordRefPrefix + ref.digest.String()
}

type ProjectTypeEnvHeadSelectionAuthorityResolutionRef struct {
	digest authority.Digest
}

func ParseProjectTypeEnvHeadSelectionAuthorityResolutionRef(
	raw string,
) (ProjectTypeEnvHeadSelectionAuthorityResolutionRef, error) {
	digest, err := parseDigestRef("authority resolution", authorityResolutionPrefix, raw)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityResolutionRef{}, err
	}
	return ProjectTypeEnvHeadSelectionAuthorityResolutionRef{digest: digest}, nil
}

func (ref ProjectTypeEnvHeadSelectionAuthorityResolutionRef) Digest() authority.Digest {
	return ref.digest
}

func (ref ProjectTypeEnvHeadSelectionAuthorityResolutionRef) String() string {
	return authorityResolutionPrefix + ref.digest.String()
}

func digestCanonical(domain string, canonical []byte) (authority.Digest, error) {
	hasher := sha256.New()
	_, _ = hasher.Write([]byte(domain))
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write(canonical)
	encoded := hex.EncodeToString(hasher.Sum(nil))
	digest, err := authority.NewDigest("sha256:" + encoded)
	if err != nil {
		return authority.Digest{}, fmt.Errorf("derive %s digest: %w", domain, err)
	}
	return digest, nil
}

func parseDigestRef(name string, prefix string, raw string) (authority.Digest, error) {
	if raw != strings.TrimSpace(raw) || !strings.HasPrefix(raw, prefix) {
		return authority.Digest{}, fmt.Errorf("%s ref must start with %q", name, prefix)
	}
	digest, err := authority.NewDigest(strings.TrimPrefix(raw, prefix))
	if err != nil {
		return authority.Digest{}, fmt.Errorf("%s ref digest: %w", name, err)
	}
	return digest, nil
}
