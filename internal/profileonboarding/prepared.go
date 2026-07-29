package profileonboarding

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

const (
	profileOnboardingVerifierPolicyDomain = "haft.profile-onboarding.verifier-policy/v1"

	profileClassifierVersion = "haft-profile-onboarding/v1"
	profilePolicyVersion     = "manual-explicit-project-profile/v2"
)

// PreparedHaftSoftwareOnboarding is the source-native, pre-SpeechAct profile
// proposal. It contains no terminal capture, SpeechAct occurrence, Permission,
// authority resolution, performed onboarding Work, or admitted profile.
type PreparedHaftSoftwareOnboarding struct {
	state *preparedHaftSoftwareOnboardingState
}

type preparedHaftSoftwareOnboardingState struct {
	token                string
	root                 projectprofile.ProjectRootV1
	refs                 dogfoodRefs
	support              dogfoodAuthoritySupport
	profileAuthorization profileauthority.PreparedAuthorization
	preparedAt           time.Time
}

// PrepareHaftSoftwareOnboarding prepares a non-binding future-Work intent. The
// fresh identity and current clock establish bounds only; this function does
// not pretend that onboarding Work or an authorizing SpeechAct happened.
func PrepareHaftSoftwareOnboarding(
	projectRoot string,
) (PreparedHaftSoftwareOnboarding, error) {
	root, err := canonicalProfileProjectRoot(projectRoot)
	if err != nil {
		return PreparedHaftSoftwareOnboarding{}, err
	}
	token, err := randomOnboardingToken()
	if err != nil {
		return PreparedHaftSoftwareOnboarding{}, err
	}
	now := time.Now().UTC().Round(0)
	return prepareHaftSoftwareOnboarding(root, token, now)
}

func (prepared PreparedHaftSoftwareOnboarding) valid() bool {
	if prepared.state == nil {
		return false
	}
	digest, digestOK := prepared.state.profileAuthorization.Digest()
	return digestOK && digest.String() != "" && !prepared.state.preparedAt.IsZero()
}

func (prepared PreparedHaftSoftwareOnboarding) ProjectRoot() (
	projectprofile.ProjectRootV1,
	bool,
) {
	if !prepared.valid() {
		return projectprofile.ProjectRootV1{}, false
	}
	return prepared.state.root, true
}

func prepareHaftSoftwareOnboarding(
	root projectprofile.ProjectRootV1,
	token string,
	now time.Time,
) (PreparedHaftSoftwareOnboarding, error) {
	return prepareProfileDeclarationAuthorization(
		root,
		token,
		now,
		profileClassifierVersion,
		profilePolicyVersion,
	)
}

func prepareProfileDeclarationAuthorization(
	root projectprofile.ProjectRootV1,
	token string,
	now time.Time,
	classifierVersion string,
	policyVersion string,
) (PreparedHaftSoftwareOnboarding, error) {
	if token == "" {
		return PreparedHaftSoftwareOnboarding{}, fmt.Errorf("profile-onboarding identity is required")
	}
	canonicalNow := now.UTC().Round(0)
	if canonicalNow.IsZero() {
		return PreparedHaftSoftwareOnboarding{}, fmt.Errorf("profile-onboarding time is required")
	}
	refs, err := newDogfoodRefs(token)
	if err != nil {
		return PreparedHaftSoftwareOnboarding{}, err
	}
	validity, err := authority.NewTimeWindow(
		canonicalNow,
		canonicalNow.Add(30*time.Minute),
	)
	if err != nil {
		return PreparedHaftSoftwareOnboarding{}, err
	}
	allowedWork, err := authority.NewTimeWindow(
		canonicalNow,
		canonicalNow.Add(25*time.Minute),
	)
	if err != nil {
		return PreparedHaftSoftwareOnboarding{}, err
	}
	allowedBasis, err := authority.NewTimeWindow(
		canonicalNow,
		canonicalNow.Add(20*time.Minute),
	)
	if err != nil {
		return PreparedHaftSoftwareOnboarding{}, err
	}
	support, err := buildProfileAuthoritySupport(
		refs,
		canonicalNow,
		validity.Until(),
		classifierVersion,
		policyVersion,
	)
	if err != nil {
		return PreparedHaftSoftwareOnboarding{}, err
	}
	verifierIdentity, err := authority.NewVerifierIdentity("kernel-verifier:profile-onboarding")
	if err != nil {
		return PreparedHaftSoftwareOnboarding{}, err
	}
	verifierVersion, err := authority.NewVerifierVersion("v1")
	if err != nil {
		return PreparedHaftSoftwareOnboarding{}, err
	}
	verifierPolicyRef, err := authority.NewVerificationPolicyRef(
		"verification-policy:profile-onboarding/manual-tty-v1",
	)
	if err != nil {
		return PreparedHaftSoftwareOnboarding{}, err
	}
	verifierPolicyHash, err := authorityDigest(
		profileOnboardingVerifierPolicyDomain,
		[]string{verifierPolicyRef.String(), root.String()},
	)
	if err != nil {
		return PreparedHaftSoftwareOnboarding{}, err
	}
	profileAuthorization, err := buildProfileAuthorization(
		refs,
		root,
		support,
		allowedWork,
		allowedBasis,
		validity,
		verifierIdentity,
		verifierVersion,
		verifierPolicyRef,
		verifierPolicyHash,
	)
	if err != nil {
		return PreparedHaftSoftwareOnboarding{}, err
	}
	state := &preparedHaftSoftwareOnboardingState{
		token:                token,
		root:                 root,
		refs:                 refs,
		support:              support,
		profileAuthorization: profileAuthorization,
		preparedAt:           canonicalNow,
	}
	prepared := PreparedHaftSoftwareOnboarding{state: state}
	if !prepared.valid() {
		return PreparedHaftSoftwareOnboarding{}, fmt.Errorf("prepared Haft software onboarding is invalid")
	}
	return prepared, nil
}

func canonicalProfileProjectRoot(raw string) (projectprofile.ProjectRootV1, error) {
	absolute, err := filepath.Abs(raw)
	if err != nil {
		return projectprofile.ProjectRootV1{}, fmt.Errorf("resolve project root: %w", err)
	}
	physical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return projectprofile.ProjectRootV1{}, fmt.Errorf("resolve physical project root: %w", err)
	}
	physical = filepath.Clean(physical)
	info, err := os.Stat(physical)
	if err != nil {
		return projectprofile.ProjectRootV1{}, fmt.Errorf("inspect project root: %w", err)
	}
	if !info.IsDir() {
		return projectprofile.ProjectRootV1{}, fmt.Errorf("project root must be a directory")
	}
	root, err := projectprofile.NewProjectRootV1(physical)
	if err != nil {
		return projectprofile.ProjectRootV1{}, fmt.Errorf("parse project root: %w", err)
	}
	return root, nil
}

func randomOnboardingToken() (string, error) {
	value := make([]byte, 16)
	_, err := rand.Read(value)
	if err != nil {
		return "", fmt.Errorf("generate profile-onboarding identity: %w", err)
	}
	return hex.EncodeToString(value), nil
}

type contentDigestWriter struct {
	hash hash.Hash
}

func newContentDigestWriter(domain string) contentDigestWriter {
	writer := contentDigestWriter{hash: sha256.New()}
	writer.add(domain)
	return writer
}

func (writer contentDigestWriter) add(value string) {
	_, _ = writer.hash.Write([]byte(fmt.Sprintf("%d:%s", len(value), value)))
}

func (writer contentDigestWriter) authorityDigest() (authority.Digest, error) {
	raw := "sha256:" + hex.EncodeToString(writer.hash.Sum(nil))
	return authority.NewDigest(raw)
}

func authorityDigest(domain string, values []string) (authority.Digest, error) {
	writer := newContentDigestWriter(domain)
	addDigestValues(writer, values, 0)
	return writer.authorityDigest()
}

func addDigestValues(writer contentDigestWriter, values []string, index int) {
	if index == len(values) {
		return
	}
	writer.add(values[index])
	addDigestValues(writer, values, index+1)
}
