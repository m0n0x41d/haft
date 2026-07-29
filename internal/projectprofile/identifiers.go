package projectprofile

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	scopeIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	sha256Pattern  = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type ScopeID struct {
	value string
}

func NewScopeID(raw string) (ScopeID, error) {
	if raw != strings.TrimSpace(raw) || !scopeIDPattern.MatchString(raw) {
		return ScopeID{}, fmt.Errorf("scope_id %q must be an opaque 1-128 character identifier, not a path", raw)
	}
	return ScopeID{value: raw}, nil
}

func (id ScopeID) String() string {
	return id.value
}

func (id ScopeID) valid() bool {
	return scopeIDPattern.MatchString(id.value)
}

type EntityRef struct {
	value string
}

func NewEntityRef(raw string) (EntityRef, error) {
	value, err := parseReference("entity_ref", raw)
	if err != nil {
		return EntityRef{}, err
	}
	return EntityRef{value: value}, nil
}

func (ref EntityRef) String() string {
	return ref.value
}

func (ref EntityRef) valid() bool {
	return validReference(ref.value)
}

type KindRef struct {
	value string
}

func NewKindRef(raw string) (KindRef, error) {
	value, err := parseReference("kind_ref", raw)
	if err != nil {
		return KindRef{}, err
	}
	return KindRef{value: value}, nil
}

func (ref KindRef) String() string {
	return ref.value
}

func (ref KindRef) valid() bool {
	return validReference(ref.value)
}

type SourceUnitRef struct {
	value string
}

func NewSourceUnitRef(raw string) (SourceUnitRef, error) {
	value, err := parseReference("source_unit_ref", raw)
	if err != nil {
		return SourceUnitRef{}, err
	}
	return SourceUnitRef{value: value}, nil
}

func (ref SourceUnitRef) String() string {
	return ref.value
}

func (ref SourceUnitRef) valid() bool {
	return validReference(ref.value)
}

type SpecSectionRef struct {
	value string
}

func NewSpecSectionRef(raw string) (SpecSectionRef, error) {
	value, err := parseReference("spec_section_ref", raw)
	if err != nil {
		return SpecSectionRef{}, err
	}
	return SpecSectionRef{value: value}, nil
}

func (ref SpecSectionRef) String() string {
	return ref.value
}

func (ref SpecSectionRef) valid() bool {
	return validReference(ref.value)
}

type ContentDigest struct {
	value string
}

func NewContentDigest(raw string) (ContentDigest, error) {
	if raw != strings.TrimSpace(raw) || !sha256Pattern.MatchString(raw) {
		return ContentDigest{}, fmt.Errorf("content digest must use canonical sha256:<64 lowercase hex> form")
	}
	return ContentDigest{value: raw}, nil
}

func (digest ContentDigest) String() string {
	return digest.value
}

func (digest ContentDigest) valid() bool {
	return sha256Pattern.MatchString(digest.value)
}

type CarrierRevision struct {
	value uint64
}

func NewCarrierRevision(value uint64) (CarrierRevision, error) {
	if value == 0 {
		return CarrierRevision{}, fmt.Errorf("carrier revision must be greater than zero")
	}
	return CarrierRevision{value: value}, nil
}

func (revision CarrierRevision) Value() uint64 {
	return revision.value
}

func (revision CarrierRevision) valid() bool {
	return revision.value > 0
}

func parseReference(name, raw string) (string, error) {
	if !validReference(raw) {
		return "", fmt.Errorf("%s must be a non-empty reference without control characters", name)
	}
	return raw, nil
}

func validReference(value string) bool {
	if value == "" || value != strings.TrimSpace(value) {
		return false
	}
	return !strings.ContainsFunc(value, unicode.IsControl)
}

func requireText(name, raw string) (string, error) {
	if raw == "" || raw != strings.TrimSpace(raw) || strings.ContainsFunc(raw, unicode.IsControl) {
		return "", fmt.Errorf("%s must be non-empty and contain no control characters", name)
	}
	return raw, nil
}
