package authority

import (
	"fmt"
	"slices"
	"time"
)

const profileDeclarationActionKind = "profile.declare.from_onboarding_candidate"

// AuthorityUseRecordRef identifies the durable record of one consumed
// profile-declaration authority use. It is a reference, not an authority
// capability.
type AuthorityUseRecordRef struct{ value string }

func NewAuthorityUseRecordRef(raw string) (AuthorityUseRecordRef, error) {
	value, err := parseAuthorityReference("AuthorityUseRecord ref", raw)
	return AuthorityUseRecordRef{value: value}, err
}

func (value AuthorityUseRecordRef) String() string { return value.value }
func (value AuthorityUseRecordRef) valid() bool    { return validAuthorityReference(value.value) }

// ProfileDeclarationAdmissionRecordRef identifies the durable result linked
// from an AuthorityUseRecord. It does not prove that the linked tuple exists or
// is integrity-valid.
type ProfileDeclarationAdmissionRecordRef struct{ value string }

func NewProfileDeclarationAdmissionRecordRef(
	raw string,
) (ProfileDeclarationAdmissionRecordRef, error) {
	value, err := parseAuthorityReference("ProfileDeclarationAdmissionRecord ref", raw)
	return ProfileDeclarationAdmissionRecordRef{value: value}, err
}

func (value ProfileDeclarationAdmissionRecordRef) String() string { return value.value }
func (value ProfileDeclarationAdmissionRecordRef) valid() bool {
	return validAuthorityReference(value.value)
}

type ProfileAdmissionAuthorityCheckKind string

const (
	ProfileAdmissionAuthoritySnapshotLoaded ProfileAdmissionAuthorityCheckKind = "snapshot_loaded"
	ProfileAdmissionAuthorityNotAdmitted    ProfileAdmissionAuthorityCheckKind = "not_admitted"
	ProfileAdmissionAuthorityCheckInvalid   ProfileAdmissionAuthorityCheckKind = "invalid"
)

const (
	profileAdmissionAdmittedVariant uint8 = 1 << iota
	profileAdmissionOriginalVariant
	profileAdmissionDeniedVariant
)

var profileAdmissionUseDecisionKindByVariant = map[uint8]ProfileAdmissionUseDecisionKind{
	profileAdmissionAdmittedVariant: ProfileAdmissionNewUseAdmitted,
	profileAdmissionOriginalVariant: ProfileAdmissionOriginalUse,
	profileAdmissionDeniedVariant:   ProfileAdmissionUseNotAdmitted,
}

// ProfileAdmissionAuthorityCheck is the closed result of loading the exact
// canonical authority state on a transaction-owned queryer. A loaded snapshot
// is read-only; callers cannot supply one to a mutation API in this package.
type ProfileAdmissionAuthorityCheck struct {
	snapshot    *ProfileAdmissionAuthoritySnapshot
	notAdmitted *NotAdmitted
}

func (value ProfileAdmissionAuthorityCheck) Kind() ProfileAdmissionAuthorityCheckKind {
	snapshotValid := value.snapshot != nil && value.snapshot.valid()
	denialValid := value.notAdmitted != nil && value.notAdmitted.valid()
	if snapshotValid && !denialValid {
		return ProfileAdmissionAuthoritySnapshotLoaded
	}
	if denialValid && !snapshotValid {
		return ProfileAdmissionAuthorityNotAdmitted
	}
	return ProfileAdmissionAuthorityCheckInvalid
}

func (value ProfileAdmissionAuthorityCheck) Snapshot() (ProfileAdmissionAuthoritySnapshot, bool) {
	if value.Kind() != ProfileAdmissionAuthoritySnapshotLoaded {
		return ProfileAdmissionAuthoritySnapshot{}, false
	}
	return *value.snapshot, true
}

func (value ProfileAdmissionAuthorityCheck) NotAdmitted() (NotAdmitted, bool) {
	if value.Kind() != ProfileAdmissionAuthorityNotAdmitted {
		return NotAdmitted{}, false
	}
	return *value.notAdmitted, true
}

type profileAdmissionAuthoritySnapshotState struct {
	presentation   Presentation
	resolution     canonicalAuthorityResolution
	judgementTime  time.Time
	currentDenials []Denial
	recordedUse    *ProfileAdmissionRecordedUse
}

// ProfileAdmissionAuthoritySnapshot is an immutable view of the exact
// presentation, resolution, current-time posture, and matching durable use
// observed through one transaction queryer. Its fields are package-owned so a
// model, CLI field, deserializer, or interface embedding cannot forge it.
type ProfileAdmissionAuthoritySnapshot struct {
	state *profileAdmissionAuthoritySnapshotState
}

func (value ProfileAdmissionAuthoritySnapshot) Presentation() (Presentation, bool) {
	if !value.valid() {
		return Presentation{}, false
	}
	return value.state.presentation, true
}

func (value ProfileAdmissionAuthoritySnapshot) Envelope() (AuthorizationEnvelope, bool) {
	presentation, ok := value.Presentation()
	if !ok {
		return AuthorizationEnvelope{}, false
	}
	return presentation.Envelope(), true
}

func (value ProfileAdmissionAuthoritySnapshot) AuthorityResolutionID() (AuthorityResolutionID, bool) {
	if !value.valid() {
		return AuthorityResolutionID{}, false
	}
	return value.state.resolution.id, true
}

func (value ProfileAdmissionAuthoritySnapshot) AuthorityResolutionDigest() (Digest, bool) {
	if !value.valid() {
		return Digest{}, false
	}
	return value.state.resolution.digest, true
}

func (value ProfileAdmissionAuthoritySnapshot) VerifierIdentity() (VerifierIdentity, bool) {
	if !value.valid() {
		return VerifierIdentity{}, false
	}
	return value.state.resolution.verifierIdentity, true
}

func (value ProfileAdmissionAuthoritySnapshot) VerifierVersion() (VerifierVersion, bool) {
	if !value.valid() {
		return VerifierVersion{}, false
	}
	return value.state.resolution.verifierVersion, true
}

func (value ProfileAdmissionAuthoritySnapshot) JudgementTime() (time.Time, bool) {
	if !value.valid() {
		return time.Time{}, false
	}
	return value.state.judgementTime, true
}

// RecordedUse exposes the already-linked historical use, when one exists, so
// the service can strictly reload the original admission result. It does not
// decide whether the incoming intent is the same intent; AssessUse does.
func (value ProfileAdmissionAuthoritySnapshot) RecordedUse() (ProfileAdmissionRecordedUse, bool) {
	if !value.valid() || value.state.recordedUse == nil {
		return ProfileAdmissionRecordedUse{}, false
	}
	return *value.state.recordedUse, true
}

func (value ProfileAdmissionAuthoritySnapshot) valid() bool {
	if value.state == nil {
		return false
	}
	presentation := value.state.presentation
	if !presentation.valid() {
		return false
	}
	resolution := value.state.resolution
	if validateCanonicalAuthorityResolution(resolution, presentation.value) != nil {
		return false
	}
	if value.state.judgementTime.IsZero() {
		return false
	}
	if !validDenials(value.state.currentDenials) {
		return false
	}
	if value.state.recordedUse != nil && !value.state.recordedUse.valid() {
		return false
	}
	return true
}

type ProfileAdmissionUseDecisionKind string

const (
	ProfileAdmissionNewUseAdmitted     ProfileAdmissionUseDecisionKind = "new_use_admitted"
	ProfileAdmissionOriginalUse        ProfileAdmissionUseDecisionKind = "original_use"
	ProfileAdmissionUseNotAdmitted     ProfileAdmissionUseDecisionKind = "not_admitted"
	ProfileAdmissionUseDecisionInvalid ProfileAdmissionUseDecisionKind = "invalid"
)

// ProfileAdmissionUseDecision distinguishes a new current use from an
// idempotent replay of the exact durable result and from a denial. It never
// writes or consumes authority.
type ProfileAdmissionUseDecision struct {
	admitted    *ProfileAdmissionAdmittedUse
	original    *ProfileAdmissionRecordedUse
	notAdmitted *NotAdmitted
}

func (value ProfileAdmissionUseDecision) Kind() ProfileAdmissionUseDecisionKind {
	admittedValid := value.admitted != nil && value.admitted.valid()
	originalValid := value.original != nil && value.original.valid()
	denialValid := value.notAdmitted != nil && value.notAdmitted.valid()
	variant := profileAdmissionVariantBit(admittedValid, profileAdmissionAdmittedVariant)
	variant |= profileAdmissionVariantBit(originalValid, profileAdmissionOriginalVariant)
	variant |= profileAdmissionVariantBit(denialValid, profileAdmissionDeniedVariant)
	kind, ok := profileAdmissionUseDecisionKindByVariant[variant]
	if !ok {
		return ProfileAdmissionUseDecisionInvalid
	}
	return kind
}

func profileAdmissionVariantBit(valid bool, bit uint8) uint8 {
	if valid {
		return bit
	}
	return 0
}

func (value ProfileAdmissionUseDecision) AdmittedUse() (ProfileAdmissionAdmittedUse, bool) {
	if value.Kind() != ProfileAdmissionNewUseAdmitted {
		return ProfileAdmissionAdmittedUse{}, false
	}
	return *value.admitted, true
}

func (value ProfileAdmissionUseDecision) OriginalUse() (ProfileAdmissionRecordedUse, bool) {
	if value.Kind() != ProfileAdmissionOriginalUse {
		return ProfileAdmissionRecordedUse{}, false
	}
	return *value.original, true
}

func (value ProfileAdmissionUseDecision) NotAdmitted() (NotAdmitted, bool) {
	if value.Kind() != ProfileAdmissionUseNotAdmitted {
		return NotAdmitted{}, false
	}
	return *value.notAdmitted, true
}

// ProfileAdmissionAdmittedUse is an opaque, transaction-snapshot-bound view of
// a currently valid and still-unused authority resolution. The profile
// admission service may use its getters while constructing its own canonical
// transaction rows; no authority mutation API accepts this value.
type ProfileAdmissionAdmittedUse struct {
	state *profileAdmissionAuthoritySnapshotState
}

func (value ProfileAdmissionAdmittedUse) Presentation() (Presentation, bool) {
	snapshot := ProfileAdmissionAuthoritySnapshot(value)
	return snapshot.Presentation()
}

func (value ProfileAdmissionAdmittedUse) Envelope() (AuthorizationEnvelope, bool) {
	snapshot := ProfileAdmissionAuthoritySnapshot(value)
	return snapshot.Envelope()
}

func (value ProfileAdmissionAdmittedUse) AuthorityResolutionID() (AuthorityResolutionID, bool) {
	snapshot := ProfileAdmissionAuthoritySnapshot(value)
	return snapshot.AuthorityResolutionID()
}

func (value ProfileAdmissionAdmittedUse) AuthorityResolutionDigest() (Digest, bool) {
	snapshot := ProfileAdmissionAuthoritySnapshot(value)
	return snapshot.AuthorityResolutionDigest()
}

func (value ProfileAdmissionAdmittedUse) VerifierIdentity() (VerifierIdentity, bool) {
	snapshot := ProfileAdmissionAuthoritySnapshot(value)
	return snapshot.VerifierIdentity()
}

func (value ProfileAdmissionAdmittedUse) VerifierVersion() (VerifierVersion, bool) {
	snapshot := ProfileAdmissionAuthoritySnapshot(value)
	return snapshot.VerifierVersion()
}

func (value ProfileAdmissionAdmittedUse) JudgementTime() (time.Time, bool) {
	snapshot := ProfileAdmissionAuthoritySnapshot(value)
	return snapshot.JudgementTime()
}

func (value ProfileAdmissionAdmittedUse) valid() bool {
	snapshot := ProfileAdmissionAuthoritySnapshot(value)
	if !snapshot.valid() {
		return false
	}
	if value.state.recordedUse != nil {
		return false
	}
	return len(value.state.currentDenials) == 0
}

// ProfileAdmissionRecordedUse is the exact durable use row observed with the
// authority snapshot. It is historical provenance, not a reusable capability.
type ProfileAdmissionRecordedUse struct {
	useRef                    AuthorityUseRecordRef
	authorityResolutionRef    AuthorityResolutionID
	authorityResolutionDigest Digest
	singleUseKey              SingleUseKey
	actionKind                ActionKind
	projectRoot               ProjectRoot
	projectBindingDigest      Digest
	envelopeDigest            Digest
	authorityRecordRef        PresentationID
	authorityRecordDigest     Digest
	admissionRequestDigest    Digest
	verifierIdentity          VerifierIdentity
	verifierVersion           VerifierVersion
	committedResultRef        ProfileDeclarationAdmissionRecordRef
	committedResultDigest     Digest
	consumedAt                time.Time
}

func (value ProfileAdmissionRecordedUse) AuthorityUseRecordRef() AuthorityUseRecordRef {
	return value.useRef
}

func (value ProfileAdmissionRecordedUse) AdmissionRequestDigest() Digest {
	return value.admissionRequestDigest
}

func (value ProfileAdmissionRecordedUse) CommittedResultRef() ProfileDeclarationAdmissionRecordRef {
	return value.committedResultRef
}

func (value ProfileAdmissionRecordedUse) CommittedResultDigest() Digest {
	return value.committedResultDigest
}

func (value ProfileAdmissionRecordedUse) ConsumedAt() time.Time {
	return value.consumedAt
}

func (value ProfileAdmissionRecordedUse) valid() bool {
	return value.useRef.valid() &&
		value.authorityResolutionRef.valid() &&
		value.authorityResolutionDigest.valid() &&
		value.singleUseKey.valid() &&
		value.actionKind.valid() &&
		value.projectRoot.valid() &&
		value.projectBindingDigest.valid() &&
		value.envelopeDigest.valid() &&
		value.authorityRecordRef.valid() &&
		value.authorityRecordDigest.valid() &&
		value.admissionRequestDigest.valid() &&
		value.verifierIdentity.valid() &&
		value.verifierVersion.valid() &&
		value.committedResultRef.valid() &&
		value.committedResultDigest.valid() &&
		!value.consumedAt.IsZero()
}

// AssessUse performs the pure consumption decision over a previously loaded
// immutable snapshot. Same key plus the same request digest replays the exact
// recorded result; a different digest conflicts; an unused snapshot must also
// be current at its judgement time.
func (value ProfileAdmissionAuthoritySnapshot) AssessUse(
	admissionRequestDigest Digest,
) ProfileAdmissionUseDecision {
	if !value.valid() {
		return ProfileAdmissionUseDecision{}
	}
	if !admissionRequestDigest.valid() {
		denial := newProfileAdmissionNotAdmitted(
			DenialInvalidRequest,
			"admission-request digest is invalid",
		)
		return ProfileAdmissionUseDecision{notAdmitted: &denial}
	}
	recordedUse := value.state.recordedUse
	if recordedUse != nil && recordedUse.admissionRequestDigest == admissionRequestDigest {
		copyOfUse := *recordedUse
		return ProfileAdmissionUseDecision{original: &copyOfUse}
	}
	if recordedUse != nil {
		denial := newProfileAdmissionNotAdmitted(
			DenialSingleUseAlreadyConsumed,
			"single-use authority is bound to a different admission-request digest",
		)
		return ProfileAdmissionUseDecision{notAdmitted: &denial}
	}
	if len(value.state.currentDenials) > 0 {
		denial := NotAdmitted{reasons: cloneDenials(value.state.currentDenials)}
		return ProfileAdmissionUseDecision{notAdmitted: &denial}
	}
	admitted := ProfileAdmissionAdmittedUse(value)
	return ProfileAdmissionUseDecision{admitted: &admitted}
}

func newProfileAdmissionNotAdmitted(code DenialCode, detail string) NotAdmitted {
	return NotAdmitted{reasons: []Denial{{code: code, detail: detail}}}
}

func cloneDenials(values []Denial) []Denial {
	return append([]Denial{}, values...)
}

func validDenials(values []Denial) bool {
	invalidIndex := slices.IndexFunc(values, func(value Denial) bool {
		return value.code == "" || value.detail == ""
	})
	return invalidIndex < 0
}

func validateProfileDeclarationRecordedUse(
	value ProfileAdmissionRecordedUse,
	presentation canonicalPresentation,
	resolution canonicalAuthorityResolution,
	judgementTime time.Time,
) error {
	err := validateHistoricalProfileDeclarationRecordedUse(
		value,
		presentation,
		resolution,
	)
	if err != nil {
		return err
	}
	if value.consumedAt.After(judgementTime) {
		return fmt.Errorf("recorded authority use is later than the transaction judgement time")
	}
	return nil
}

func validateHistoricalProfileDeclarationRecordedUse(
	value ProfileAdmissionRecordedUse,
	presentation canonicalPresentation,
	resolution canonicalAuthorityResolution,
) error {
	if !value.valid() {
		return fmt.Errorf("recorded authority use is structurally invalid")
	}
	envelopeDigest, err := presentation.envelope.Digest()
	if err != nil {
		return err
	}
	projectBindingDigest, err := ProjectBindingDigest(
		presentation.envelope.actionKind,
		presentation.envelope.projectRoot,
	)
	if err != nil {
		return err
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: value.authorityResolutionRef == resolution.id, name: "authority-resolution ref"},
		{matches: value.authorityResolutionDigest == resolution.digest, name: "authority-resolution digest"},
		{matches: value.singleUseKey == presentation.envelope.singleUseKey, name: "single-use key"},
		{matches: value.actionKind == presentation.envelope.actionKind, name: "action kind"},
		{matches: value.projectRoot == presentation.envelope.projectRoot, name: "project root"},
		{matches: value.projectBindingDigest == projectBindingDigest, name: "project-binding digest"},
		{matches: value.envelopeDigest == envelopeDigest, name: "envelope digest"},
		{matches: value.authorityRecordRef == presentation.id, name: "authority-record ref"},
		{matches: value.authorityRecordDigest == presentation.digest, name: "authority-record digest"},
		{matches: value.verifierIdentity == resolution.verifierIdentity, name: "verifier identity"},
		{matches: value.verifierVersion == resolution.verifierVersion, name: "verifier version"},
	}
	mismatchIndex := slices.IndexFunc(checks, func(check struct {
		matches bool
		name    string
	}) bool {
		return !check.matches
	})
	if mismatchIndex >= 0 {
		return fmt.Errorf("recorded authority use has a mismatched %s", checks[mismatchIndex].name)
	}
	if value.consumedAt.Before(resolution.resolvedAt) {
		return fmt.Errorf("recorded authority use predates its authority resolution")
	}
	if !value.consumedAt.Before(resolution.validUntil) {
		return fmt.Errorf("recorded authority use is outside its resolution validity")
	}
	if !presentation.envelope.authorizationValidityWindow.Contains(value.consumedAt) {
		return fmt.Errorf("recorded authority use is outside its authorization validity")
	}
	return nil
}
