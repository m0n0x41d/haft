package authority

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"slices"
	"time"
)

const (
	authorizationEnvelopeDigestDomain = "haft.authority.authorization-envelope/v1"
	profileDeclarationActionKindValue = "profile.declare.from_onboarding_candidate"
)

func ProfileDeclarationActionKind() ActionKind {
	return ActionKind{value: profileDeclarationActionKindValue}
}

type TimeWindow struct {
	from  time.Time
	until time.Time
}

func NewTimeWindow(from time.Time, until time.Time) (TimeWindow, error) {
	canonicalFrom := canonicalAuthorityTime(from)
	canonicalUntil := canonicalAuthorityTime(until)
	if canonicalFrom.IsZero() || !canonicalUntil.After(canonicalFrom) {
		return TimeWindow{}, fmt.Errorf("time window requires a non-zero start and a later end")
	}
	return TimeWindow{from: canonicalFrom, until: canonicalUntil}, nil
}

func (value TimeWindow) From() time.Time  { return value.from }
func (value TimeWindow) Until() time.Time { return value.until }

func (value TimeWindow) Contains(instant time.Time) bool {
	canonical := canonicalAuthorityTime(instant)
	return !canonical.Before(value.from) && canonical.Before(value.until)
}

func (value TimeWindow) valid() bool {
	return !value.from.IsZero() && value.until.After(value.from)
}

type AuthorizationEnvelope struct {
	actionKind                  ActionKind
	projectRoot                 ProjectRoot
	profileAuthor               RoleAssignmentRef
	profileAuthorDigest         Digest
	methodDescription           MethodDescriptionRef
	methodDescriptionDigest     Digest
	methodContract              MethodContractRef
	methodContractDigest        Digest
	classifierVersion           ClassifierVersion
	policyVersion               PolicyVersion
	sessionRef                  SessionRef
	allowedWorkWindow           TimeWindow
	allowedBasisObservation     TimeWindow
	authorizationValidityWindow TimeWindow
	singleUseKey                SingleUseKey
}

type AuthorizationEnvelopeBuilder struct {
	value AuthorizationEnvelope
}

func NewAuthorizationEnvelopeBuilder(
	actionKind ActionKind,
	projectRoot ProjectRoot,
) AuthorizationEnvelopeBuilder {
	return AuthorizationEnvelopeBuilder{
		value: AuthorizationEnvelope{
			actionKind:  actionKind,
			projectRoot: projectRoot,
		},
	}
}

func (builder AuthorizationEnvelopeBuilder) ForProfileAuthor(
	ref RoleAssignmentRef,
	digest Digest,
) AuthorizationEnvelopeBuilder {
	// The presentation consumes this pre-existing immutable assignment. It
	// neither institutes the assignment nor extends its session or validity.
	builder.value.profileAuthor = ref
	builder.value.profileAuthorDigest = digest
	return builder
}

func (builder AuthorizationEnvelopeBuilder) ForMethodDescription(
	ref MethodDescriptionRef,
	digest Digest,
) AuthorizationEnvelopeBuilder {
	builder.value.methodDescription = ref
	builder.value.methodDescriptionDigest = digest
	return builder
}

func (builder AuthorizationEnvelopeBuilder) UnderMethodContract(
	ref MethodContractRef,
	digest Digest,
) AuthorizationEnvelopeBuilder {
	builder.value.methodContract = ref
	builder.value.methodContractDigest = digest
	return builder
}

func (builder AuthorizationEnvelopeBuilder) WithClassifier(
	classifierVersion ClassifierVersion,
	policyVersion PolicyVersion,
) AuthorizationEnvelopeBuilder {
	builder.value.classifierVersion = classifierVersion
	builder.value.policyVersion = policyVersion
	return builder
}

func (builder AuthorizationEnvelopeBuilder) InSession(
	ref SessionRef,
) AuthorizationEnvelopeBuilder {
	builder.value.sessionRef = ref
	return builder
}

func (builder AuthorizationEnvelopeBuilder) AllowWorkWithin(
	window TimeWindow,
) AuthorizationEnvelopeBuilder {
	builder.value.allowedWorkWindow = window
	return builder
}

func (builder AuthorizationEnvelopeBuilder) AllowBasisObservationWithin(
	window TimeWindow,
) AuthorizationEnvelopeBuilder {
	builder.value.allowedBasisObservation = window
	return builder
}

func (builder AuthorizationEnvelopeBuilder) ValidWithin(
	window TimeWindow,
) AuthorizationEnvelopeBuilder {
	builder.value.authorizationValidityWindow = window
	return builder
}

func (builder AuthorizationEnvelopeBuilder) SingleUse(
	key SingleUseKey,
) AuthorizationEnvelopeBuilder {
	builder.value.singleUseKey = key
	return builder
}

func (builder AuthorizationEnvelopeBuilder) Build() (AuthorizationEnvelope, error) {
	if err := validateAuthorizationEnvelope(builder.value); err != nil {
		return AuthorizationEnvelope{}, err
	}
	return builder.value, nil
}

func (value AuthorizationEnvelope) ActionKind() ActionKind {
	return value.actionKind
}

func (value AuthorizationEnvelope) ProjectRoot() ProjectRoot {
	return value.projectRoot
}

func (value AuthorizationEnvelope) ProfileAuthor() RoleAssignmentRef {
	return value.profileAuthor
}

func (value AuthorizationEnvelope) ProfileAuthorDigest() Digest {
	return value.profileAuthorDigest
}

func (value AuthorizationEnvelope) MethodDescription() MethodDescriptionRef {
	return value.methodDescription
}

func (value AuthorizationEnvelope) MethodDescriptionDigest() Digest {
	return value.methodDescriptionDigest
}

func (value AuthorizationEnvelope) MethodContract() MethodContractRef {
	return value.methodContract
}

func (value AuthorizationEnvelope) MethodContractDigest() Digest {
	return value.methodContractDigest
}

func (value AuthorizationEnvelope) ClassifierVersion() ClassifierVersion {
	return value.classifierVersion
}

func (value AuthorizationEnvelope) PolicyVersion() PolicyVersion {
	return value.policyVersion
}

func (value AuthorizationEnvelope) SessionRef() SessionRef {
	return value.sessionRef
}

func (value AuthorizationEnvelope) AllowedWorkWindow() TimeWindow {
	return value.allowedWorkWindow
}

func (value AuthorizationEnvelope) AllowedBasisObservationWindow() TimeWindow {
	return value.allowedBasisObservation
}

func (value AuthorizationEnvelope) AuthorizationValidityWindow() TimeWindow {
	return value.authorizationValidityWindow
}

func (value AuthorizationEnvelope) SingleUseKey() SingleUseKey {
	return value.singleUseKey
}

func (value AuthorizationEnvelope) Digest() (Digest, error) {
	if err := validateAuthorizationEnvelope(value); err != nil {
		return Digest{}, err
	}
	writer := newAuthorityDigestWriter(authorizationEnvelopeDigestDomain)
	writer.add(value.actionKind.String())
	writer.add(value.projectRoot.String())
	writer.add(value.profileAuthor.String())
	writer.add(value.profileAuthorDigest.String())
	writer.add(value.methodDescription.String())
	writer.add(value.methodDescriptionDigest.String())
	writer.add(value.methodContract.String())
	writer.add(value.methodContractDigest.String())
	writer.add(value.classifierVersion.String())
	writer.add(value.policyVersion.String())
	writer.add(value.sessionRef.String())
	writer.add(formatAuthorityTime(value.allowedWorkWindow.from))
	writer.add(formatAuthorityTime(value.allowedWorkWindow.until))
	writer.add(formatAuthorityTime(value.allowedBasisObservation.from))
	writer.add(formatAuthorityTime(value.allowedBasisObservation.until))
	writer.add(formatAuthorityTime(value.authorizationValidityWindow.from))
	writer.add(formatAuthorityTime(value.authorizationValidityWindow.until))
	writer.add(value.singleUseKey.String())
	return writer.digest(), nil
}

func validateAuthorizationEnvelope(value AuthorizationEnvelope) error {
	checks := []struct {
		valid  bool
		reason string
	}{
		{valid: value.actionKind.valid(), reason: "action kind is invalid"},
		{valid: value.projectRoot.valid(), reason: "project root is invalid"},
		{valid: value.profileAuthor.valid(), reason: "profile-author RoleAssignment ref is invalid"},
		{valid: value.profileAuthorDigest.valid(), reason: "profile-author RoleAssignment digest is invalid"},
		{valid: value.methodDescription.valid(), reason: "MethodDescription ref is invalid"},
		{valid: value.methodDescriptionDigest.valid(), reason: "MethodDescription digest is invalid"},
		{valid: value.methodContract.valid(), reason: "MethodContract ref is invalid"},
		{valid: value.methodContractDigest.valid(), reason: "MethodContract digest is invalid"},
		{valid: value.classifierVersion.valid(), reason: "classifier version is invalid"},
		{valid: value.policyVersion.valid(), reason: "policy version is invalid"},
		{valid: value.sessionRef.valid(), reason: "session ref is invalid"},
		{valid: value.allowedWorkWindow.valid(), reason: "allowed Work window is invalid"},
		{valid: value.allowedBasisObservation.valid(), reason: "allowed basis-observation window is invalid"},
		{valid: value.authorizationValidityWindow.valid(), reason: "authorization validity window is invalid"},
		{valid: value.singleUseKey.valid(), reason: "single-use key is invalid"},
		{
			valid: value.authorizationValidityWindow.Contains(value.allowedWorkWindow.from) &&
				!value.allowedWorkWindow.until.After(value.authorizationValidityWindow.until),
			reason: "authorization validity window must cover the allowed Work window",
		},
		{
			valid: value.authorizationValidityWindow.Contains(value.allowedBasisObservation.from) &&
				!value.allowedBasisObservation.until.After(value.authorizationValidityWindow.until),
			reason: "authorization validity window must cover the basis-observation window",
		},
	}
	invalidIndex := slices.IndexFunc(checks, func(check struct {
		valid  bool
		reason string
	}) bool {
		return !check.valid
	})
	if invalidIndex >= 0 {
		return fmt.Errorf("%s", checks[invalidIndex].reason)
	}
	return nil
}

type authorityDigestWriter struct {
	hash hash.Hash
}

func newAuthorityDigestWriter(domain string) authorityDigestWriter {
	writer := authorityDigestWriter{hash: sha256.New()}
	writer.add(domain)
	return writer
}

func (writer authorityDigestWriter) add(value string) {
	_, _ = writer.hash.Write([]byte(fmt.Sprintf("%d:%s", len(value), value)))
}

func (writer authorityDigestWriter) digest() Digest {
	value := "sha256:" + hex.EncodeToString(writer.hash.Sum(nil))
	return Digest{value: value}
}

func canonicalAuthorityTime(value time.Time) time.Time {
	return value.Round(0).UTC()
}

func formatAuthorityTime(value time.Time) string {
	return canonicalAuthorityTime(value).Format(time.RFC3339Nano)
}

func parseAuthorityTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse canonical authority time: %w", err)
	}
	canonical := canonicalAuthorityTime(parsed)
	if value != formatAuthorityTime(canonical) {
		return time.Time{}, fmt.Errorf("authority time must use canonical UTC RFC3339Nano form")
	}
	return canonical, nil
}
