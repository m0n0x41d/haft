package authority

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	authorityTokenPattern  = regexp.MustCompile(`^[a-z][a-z0-9._:-]{0,127}$`)
	authorityDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type PresentationID struct{ value string }

func NewPresentationID(raw string) (PresentationID, error) {
	value, err := parseAuthorityToken("presentation ID", raw)
	return PresentationID{value: value}, err
}

func (value PresentationID) String() string { return value.value }
func (value PresentationID) valid() bool    { return authorityTokenPattern.MatchString(value.value) }

type AuthorityResolutionID struct{ value string }

func NewAuthorityResolutionID(raw string) (AuthorityResolutionID, error) {
	value, err := parseAuthorityToken("authority-resolution ID", raw)
	return AuthorityResolutionID{value: value}, err
}

func (value AuthorityResolutionID) String() string { return value.value }
func (value AuthorityResolutionID) valid() bool {
	return authorityTokenPattern.MatchString(value.value)
}

type SpeechActRef struct{ value string }

func NewSpeechActRef(raw string) (SpeechActRef, error) {
	value, err := parseAuthorityReference("SpeechAct ref", raw)
	return SpeechActRef{value: value}, err
}

func (value SpeechActRef) String() string { return value.value }
func (value SpeechActRef) valid() bool    { return validAuthorityReference(value.value) }

type AuthorizationContentRef struct{ value string }

func NewAuthorizationContentRef(raw string) (AuthorizationContentRef, error) {
	value, err := parseAuthorityReference("authorization-content ref", raw)
	return AuthorizationContentRef{value: value}, err
}

func (value AuthorizationContentRef) String() string { return value.value }
func (value AuthorizationContentRef) valid() bool    { return validAuthorityReference(value.value) }

type PermissionRef struct{ value string }

func NewPermissionRef(raw string) (PermissionRef, error) {
	value, err := parseAuthorityReference("permission ref", raw)
	return PermissionRef{value: value}, err
}

func (value PermissionRef) String() string { return value.value }
func (value PermissionRef) valid() bool    { return validAuthorityReference(value.value) }

type ProfileAdmissionPredicateRef struct{ value string }

func NewProfileAdmissionPredicateRef(raw string) (ProfileAdmissionPredicateRef, error) {
	value, err := parseAuthorityReference("profile-admission predicate ref", raw)
	return ProfileAdmissionPredicateRef{value: value}, err
}

func (value ProfileAdmissionPredicateRef) String() string { return value.value }
func (value ProfileAdmissionPredicateRef) valid() bool {
	return validAuthorityReference(value.value)
}

type ContextPolicyRef struct{ value string }

func NewContextPolicyRef(raw string) (ContextPolicyRef, error) {
	value, err := parseAuthorityReference("context-policy ref", raw)
	return ContextPolicyRef{value: value}, err
}

func (value ContextPolicyRef) String() string { return value.value }
func (value ContextPolicyRef) valid() bool    { return validAuthorityReference(value.value) }

type VerificationPolicyRef struct{ value string }

func NewVerificationPolicyRef(raw string) (VerificationPolicyRef, error) {
	value, err := parseAuthorityReference("verification-policy ref", raw)
	return VerificationPolicyRef{value: value}, err
}

func (value VerificationPolicyRef) String() string { return value.value }
func (value VerificationPolicyRef) valid() bool    { return validAuthorityReference(value.value) }

type VerifierIdentity struct{ value string }

func NewVerifierIdentity(raw string) (VerifierIdentity, error) {
	value, err := parseAuthorityReference("verifier identity", raw)
	return VerifierIdentity{value: value}, err
}

func (value VerifierIdentity) String() string { return value.value }
func (value VerifierIdentity) valid() bool    { return validAuthorityReference(value.value) }

type VerifierVersion struct{ value string }

func NewVerifierVersion(raw string) (VerifierVersion, error) {
	value, err := parseAuthorityReference("verifier version", raw)
	return VerifierVersion{value: value}, err
}

func (value VerifierVersion) String() string { return value.value }
func (value VerifierVersion) valid() bool    { return validAuthorityReference(value.value) }

type RoleAssignmentRef struct{ value string }

func NewRoleAssignmentRef(raw string) (RoleAssignmentRef, error) {
	value, err := parseAuthorityReference("RoleAssignment ref", raw)
	return RoleAssignmentRef{value: value}, err
}

func (value RoleAssignmentRef) String() string { return value.value }
func (value RoleAssignmentRef) valid() bool    { return validAuthorityReference(value.value) }

type MethodDescriptionRef struct{ value string }

func NewMethodDescriptionRef(raw string) (MethodDescriptionRef, error) {
	value, err := parseAuthorityReference("MethodDescription ref", raw)
	return MethodDescriptionRef{value: value}, err
}

func (value MethodDescriptionRef) String() string { return value.value }
func (value MethodDescriptionRef) valid() bool    { return validAuthorityReference(value.value) }

type MethodContractRef struct{ value string }

func NewMethodContractRef(raw string) (MethodContractRef, error) {
	value, err := parseAuthorityReference("MethodContract ref", raw)
	return MethodContractRef{value: value}, err
}

func (value MethodContractRef) String() string { return value.value }
func (value MethodContractRef) valid() bool    { return validAuthorityReference(value.value) }

type SessionRef struct{ value string }

func NewSessionRef(raw string) (SessionRef, error) {
	value, err := parseAuthorityReference("session ref", raw)
	return SessionRef{value: value}, err
}

func (value SessionRef) String() string { return value.value }
func (value SessionRef) valid() bool    { return validAuthorityReference(value.value) }

type SingleUseKey struct{ value string }

func NewSingleUseKey(raw string) (SingleUseKey, error) {
	value, err := parseAuthorityToken("single-use key", raw)
	return SingleUseKey{value: value}, err
}

func (value SingleUseKey) String() string { return value.value }
func (value SingleUseKey) valid() bool    { return authorityTokenPattern.MatchString(value.value) }

type ActionKind struct{ value string }

func NewActionKind(raw string) (ActionKind, error) {
	value, err := parseAuthorityToken("action kind", raw)
	return ActionKind{value: value}, err
}

func (value ActionKind) String() string { return value.value }
func (value ActionKind) valid() bool    { return authorityTokenPattern.MatchString(value.value) }

type ProjectRoot struct{ value string }

func NewProjectRoot(raw string) (ProjectRoot, error) {
	if !validProjectRoot(raw) {
		return ProjectRoot{}, fmt.Errorf("project root must be a canonical absolute path")
	}
	return ProjectRoot{value: raw}, nil
}

func (value ProjectRoot) String() string { return value.value }
func (value ProjectRoot) valid() bool    { return validProjectRoot(value.value) }

type ClassifierVersion struct{ value string }

func NewClassifierVersion(raw string) (ClassifierVersion, error) {
	value, err := parseAuthorityReference("classifier version", raw)
	return ClassifierVersion{value: value}, err
}

func (value ClassifierVersion) String() string { return value.value }
func (value ClassifierVersion) valid() bool    { return validAuthorityReference(value.value) }

type PolicyVersion struct{ value string }

func NewPolicyVersion(raw string) (PolicyVersion, error) {
	value, err := parseAuthorityReference("policy version", raw)
	return PolicyVersion{value: value}, err
}

func (value PolicyVersion) String() string { return value.value }
func (value PolicyVersion) valid() bool    { return validAuthorityReference(value.value) }

type Digest struct{ value string }

func NewDigest(raw string) (Digest, error) {
	if !authorityDigestPattern.MatchString(raw) {
		return Digest{}, fmt.Errorf("digest must use canonical sha256:<64 lowercase hex> form")
	}
	return Digest{value: raw}, nil
}

func (value Digest) String() string { return value.value }
func (value Digest) valid() bool    { return authorityDigestPattern.MatchString(value.value) }

func parseAuthorityToken(name string, raw string) (string, error) {
	if !authorityTokenPattern.MatchString(raw) {
		return "", fmt.Errorf("%s must be a canonical lowercase token", name)
	}
	return raw, nil
}

func parseAuthorityReference(name string, raw string) (string, error) {
	if !validAuthorityReference(raw) {
		return "", fmt.Errorf("%s must be non-empty, canonical text without control characters", name)
	}
	return raw, nil
}

func validAuthorityReference(value string) bool {
	return value != "" &&
		value == strings.TrimSpace(value) &&
		len(value) <= 1024 &&
		!strings.ContainsFunc(value, unicode.IsControl)
}

func validProjectRoot(value string) bool {
	return validAuthorityReference(value) &&
		filepath.IsAbs(value) &&
		filepath.Clean(value) == value
}
