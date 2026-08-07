package profileauthority

import (
	"fmt"
	"strings"
	"unicode"
)

// EvidenceClaimRef is the ClaimIdRef used to adjudicate the profile
// permission. It is deliberately not an arbitrary reviewed-object ref.
type EvidenceClaimRef struct {
	value string
}

func NewEvidenceClaimRef(raw string) (EvidenceClaimRef, error) {
	if !validReference(raw) || !strings.HasPrefix(raw, "E-") {
		return EvidenceClaimRef{}, fmt.Errorf(
			"profile authority evidence must be a canonical E-* ClaimIdRef",
		)
	}
	return EvidenceClaimRef{value: raw}, nil
}

func (ref EvidenceClaimRef) String() string {
	return ref.value
}

func (ref EvidenceClaimRef) valid() bool {
	return validReference(ref.value) && strings.HasPrefix(ref.value, "E-")
}

// CarrierClassRef names the expected class of an adjudication carrier. The
// actual terminal-capture carrier occurrence is bound separately on the
// instituting SpeechAct source.
type CarrierClassRef struct {
	value string
}

func NewCarrierClassRef(raw string) (CarrierClassRef, error) {
	if !validReference(raw) || !strings.HasPrefix(raw, "carrier-class:") {
		return CarrierClassRef{}, fmt.Errorf(
			"profile authority carrier class must use carrier-class:* form",
		)
	}
	return CarrierClassRef{value: raw}, nil
}

func (ref CarrierClassRef) String() string {
	return ref.value
}

func (ref CarrierClassRef) valid() bool {
	return validReference(ref.value) && strings.HasPrefix(ref.value, "carrier-class:")
}

// EnactabilityPredicateRef identifies the profile-domain A-* predicate used to
// judge whether the exact current Permission/basis/project/action closure is
// enactable before Work. It does not assert that the later candidate admission
// has succeeded and is not part of the generic SpeechAct source vocabulary.
type EnactabilityPredicateRef struct {
	value string
}

func NewEnactabilityPredicateRef(raw string) (EnactabilityPredicateRef, error) {
	if !validReference(raw) || !strings.HasPrefix(raw, "A-") {
		return EnactabilityPredicateRef{}, fmt.Errorf(
			"profile enactability predicate must be a canonical A-* ClaimIdRef",
		)
	}
	return EnactabilityPredicateRef{value: raw}, nil
}

func (ref EnactabilityPredicateRef) String() string {
	return ref.value
}

func (ref EnactabilityPredicateRef) valid() bool {
	return validReference(ref.value) && strings.HasPrefix(ref.value, "A-")
}

// BasisRef addresses exactly one four-ref profile authority closure.
type BasisRef struct {
	value string
}

func NewBasisRef(raw string) (BasisRef, error) {
	if !validReference(raw) || !strings.HasPrefix(raw, "profile-authority-basis:") {
		return BasisRef{}, fmt.Errorf(
			"profile authority basis ref must use profile-authority-basis:* form",
		)
	}
	return BasisRef{value: raw}, nil
}

func (ref BasisRef) String() string {
	return ref.value
}

func (ref BasisRef) valid() bool {
	return validReference(ref.value) && strings.HasPrefix(ref.value, "profile-authority-basis:")
}

// ProfileDeclarationAuthorityResolutionRef addresses one immutable,
// context-local evaluation of the profile-declaration MAY permission. It is a
// reference to an evaluation record, not a permission, capability, admission,
// performed Work, or single-use token.
type ProfileDeclarationAuthorityResolutionRef struct {
	value string
}

func NewProfileDeclarationAuthorityResolutionRef(
	raw string,
) (ProfileDeclarationAuthorityResolutionRef, error) {
	const prefix = "profile-authority-resolution:"
	hasPayload := strings.TrimPrefix(raw, prefix) != ""
	if !validReference(raw) || !strings.HasPrefix(raw, prefix) || !hasPayload {
		return ProfileDeclarationAuthorityResolutionRef{}, fmt.Errorf(
			"profile authority resolution ref must use profile-authority-resolution:* form",
		)
	}
	return ProfileDeclarationAuthorityResolutionRef{value: raw}, nil
}

func (ref ProfileDeclarationAuthorityResolutionRef) String() string {
	return ref.value
}

func (ref ProfileDeclarationAuthorityResolutionRef) valid() bool {
	const prefix = "profile-authority-resolution:"
	return validReference(ref.value) &&
		strings.HasPrefix(ref.value, prefix) &&
		strings.TrimPrefix(ref.value, prefix) != ""
}

// ProfileDeclarationAuthorityUseRef addresses the durable historical fact
// that one exact profile-declaration authority was consumed. It is not the
// single-use key and does not carry authority.
type ProfileDeclarationAuthorityUseRef struct {
	value string
}

func NewProfileDeclarationAuthorityUseRef(
	raw string,
) (ProfileDeclarationAuthorityUseRef, error) {
	const prefix = "profile-authority-use:"
	hasPayload := strings.TrimPrefix(raw, prefix) != ""
	if !validReference(raw) || !strings.HasPrefix(raw, prefix) || !hasPayload {
		return ProfileDeclarationAuthorityUseRef{}, fmt.Errorf(
			"profile authority use ref must use profile-authority-use:* form",
		)
	}
	return ProfileDeclarationAuthorityUseRef{value: raw}, nil
}

func (ref ProfileDeclarationAuthorityUseRef) String() string {
	return ref.value
}

func (ref ProfileDeclarationAuthorityUseRef) valid() bool {
	const prefix = "profile-authority-use:"
	return validReference(ref.value) &&
		strings.HasPrefix(ref.value, prefix) &&
		strings.TrimPrefix(ref.value, prefix) != ""
}

// CommittedProfileAdmissionRef is the profile-authority layer's strong
// reference to the already-canonical profile-admission result committed in the
// same transaction as an AuthorityUseRecord. The project-profile layer owns
// the referred object and must exact-join it at the persistence boundary.
type CommittedProfileAdmissionRef struct {
	value string
}

func NewCommittedProfileAdmissionRef(
	raw string,
) (CommittedProfileAdmissionRef, error) {
	if !validReference(raw) {
		return CommittedProfileAdmissionRef{}, fmt.Errorf(
			"committed profile admission ref is invalid",
		)
	}
	return CommittedProfileAdmissionRef{value: raw}, nil
}

func (ref CommittedProfileAdmissionRef) String() string {
	return ref.value
}

func (ref CommittedProfileAdmissionRef) valid() bool {
	return validReference(ref.value)
}

func validReference(value string) bool {
	return value != "" &&
		value == strings.TrimSpace(value) &&
		len(value) <= 1024 &&
		!strings.ContainsFunc(value, unicode.IsControl)
}
