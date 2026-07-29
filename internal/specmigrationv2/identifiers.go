package specmigrationv2

import (
	"crypto/sha256"
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode"
)

var (
	opaqueIDPattern          = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,255}$`)
	sourceSectionIDPattern   = regexp.MustCompile(`^ES(?:\.[A-Za-z0-9][A-Za-z0-9_-]*)+\.[0-9]{3}$`)
	targetSectionIDPattern   = regexp.MustCompile(`^SS(?:\.[A-Za-z0-9][A-Za-z0-9_-]*)+\.[0-9]{3}$`)
	targetAtomicClaimPattern = regexp.MustCompile(`^(SS(?:\.[A-Za-z0-9][A-Za-z0-9_-]*)+\.[0-9]{3})\.([LADE][1-9][0-9]*)$`)
	sha256Pattern            = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type MigrationPacketID struct {
	value string
}

func NewMigrationPacketID(raw string) (MigrationPacketID, error) {
	value, err := parseOpaqueID("migration packet ID", raw)
	if err != nil {
		return MigrationPacketID{}, err
	}
	return MigrationPacketID{value: value}, nil
}

func (id MigrationPacketID) String() string {
	return id.value
}

func (id MigrationPacketID) valid() bool {
	return opaqueIDPattern.MatchString(id.value)
}

type SourceCarrierID struct {
	value string
}

func NewSourceCarrierID(raw string) (SourceCarrierID, error) {
	value, err := parseCarrierID("source carrier ID", raw)
	if err != nil {
		return SourceCarrierID{}, err
	}
	return SourceCarrierID{value: value}, nil
}

func (id SourceCarrierID) String() string {
	return id.value
}

func (id SourceCarrierID) valid() bool {
	return validCarrierID(id.value)
}

type TargetCarrierID struct {
	value string
}

func NewTargetCarrierID(raw string) (TargetCarrierID, error) {
	value, err := parseCarrierID("target carrier ID", raw)
	if err != nil {
		return TargetCarrierID{}, err
	}
	return TargetCarrierID{value: value}, nil
}

func (id TargetCarrierID) String() string {
	return id.value
}

func (id TargetCarrierID) valid() bool {
	return validCarrierID(id.value)
}

type ArchiveCarrierID struct {
	value string
}

func NewArchiveCarrierID(raw string) (ArchiveCarrierID, error) {
	value, err := parseCarrierID("archive carrier ID", raw)
	if err != nil {
		return ArchiveCarrierID{}, err
	}
	return ArchiveCarrierID{value: value}, nil
}

func (id ArchiveCarrierID) String() string {
	return id.value
}

func (id ArchiveCarrierID) valid() bool {
	return validCarrierID(id.value)
}

type SourceSectionID struct {
	value string
}

func NewSourceSectionID(raw string) (SourceSectionID, error) {
	if raw != strings.TrimSpace(raw) || !sourceSectionIDPattern.MatchString(raw) {
		return SourceSectionID{}, fmt.Errorf("source section ID %q must use ES.<name>.<nnn> form", raw)
	}
	return SourceSectionID{value: raw}, nil
}

func (id SourceSectionID) String() string {
	return id.value
}

func (id SourceSectionID) valid() bool {
	return sourceSectionIDPattern.MatchString(id.value)
}

type TargetSectionID struct {
	value string
}

func NewTargetSectionID(raw string) (TargetSectionID, error) {
	if raw != strings.TrimSpace(raw) || !targetSectionIDPattern.MatchString(raw) {
		return TargetSectionID{}, fmt.Errorf("target section ID %q must use SS.<name>.<nnn> form", raw)
	}
	return TargetSectionID{value: raw}, nil
}

func (id TargetSectionID) String() string {
	return id.value
}

func (id TargetSectionID) valid() bool {
	return targetSectionIDPattern.MatchString(id.value)
}

type TargetAtomicClaimID struct {
	value   string
	section TargetSectionID
}

func NewTargetAtomicClaimID(raw string) (TargetAtomicClaimID, error) {
	if raw != strings.TrimSpace(raw) {
		return TargetAtomicClaimID{}, fmt.Errorf("target atomic claim ID %q must be canonical without surrounding whitespace", raw)
	}
	matches := targetAtomicClaimPattern.FindStringSubmatch(raw)
	if len(matches) != 3 {
		return TargetAtomicClaimID{}, fmt.Errorf("target atomic claim ID %q must use SS.<section>.<nnn>.<L|A|D|E><n> form", raw)
	}
	section, err := NewTargetSectionID(matches[1])
	if err != nil {
		return TargetAtomicClaimID{}, err
	}
	return TargetAtomicClaimID{value: raw, section: section}, nil
}

func (id TargetAtomicClaimID) String() string {
	return id.value
}

func (id TargetAtomicClaimID) Section() TargetSectionID {
	return id.section
}

func (id TargetAtomicClaimID) valid() bool {
	matches := targetAtomicClaimPattern.FindStringSubmatch(id.value)
	return len(matches) == 3 && id.section.valid() && matches[1] == id.section.String()
}

type OutsideCarrierID struct {
	value string
}

func NewOutsideCarrierID(raw string) (OutsideCarrierID, error) {
	value, err := parseOpaqueID("outside carrier ID", raw)
	if err != nil {
		return OutsideCarrierID{}, err
	}
	return OutsideCarrierID{value: value}, nil
}

func (id OutsideCarrierID) String() string {
	return id.value
}

func (id OutsideCarrierID) valid() bool {
	return opaqueIDPattern.MatchString(id.value)
}

type SHA256 struct {
	value string
}

func NewSHA256(raw string) (SHA256, error) {
	if raw != strings.TrimSpace(raw) || !sha256Pattern.MatchString(raw) {
		return SHA256{}, fmt.Errorf("digest must use canonical sha256:<64 lowercase hex> form")
	}
	return SHA256{value: raw}, nil
}

func DigestBytes(value []byte) SHA256 {
	sum := sha256.Sum256(value)
	encoded := fmt.Sprintf("%x", sum)
	return SHA256{value: "sha256:" + encoded}
}

func (digest SHA256) String() string {
	return digest.value
}

func (digest SHA256) Equal(other SHA256) bool {
	return digest.value == other.value
}

func (digest SHA256) valid() bool {
	return sha256Pattern.MatchString(digest.value)
}

type SourceDigest struct {
	value SHA256
}

type PacketDigest struct {
	value SHA256
}

func NewPacketDigest(raw string) (PacketDigest, error) {
	value, err := NewSHA256(raw)
	if err != nil {
		return PacketDigest{}, err
	}
	return PacketDigest{value: value}, nil
}

func (digest PacketDigest) String() string {
	return digest.value.String()
}

func (digest PacketDigest) Equal(other PacketDigest) bool {
	return digest.value.Equal(other.value)
}

func (digest PacketDigest) valid() bool {
	return digest.value.valid()
}

func NewSourceDigest(raw string) (SourceDigest, error) {
	value, err := NewSHA256(raw)
	if err != nil {
		return SourceDigest{}, err
	}
	return SourceDigest{value: value}, nil
}

func SourceDigestOf(value []byte) SourceDigest {
	digest := DigestBytes(value)
	return SourceDigest{value: digest}
}

func (digest SourceDigest) String() string {
	return digest.value.String()
}

func (digest SourceDigest) Equal(other SourceDigest) bool {
	return digest.value.Equal(other.value)
}

func (digest SourceDigest) equalBytes(value []byte) bool {
	observed := SourceDigestOf(value)
	return digest.value.Equal(observed.value)
}

func (digest SourceDigest) valid() bool {
	return digest.value.valid()
}

type TargetDigest struct {
	value SHA256
}

func NewTargetDigest(raw string) (TargetDigest, error) {
	value, err := NewSHA256(raw)
	if err != nil {
		return TargetDigest{}, err
	}
	return TargetDigest{value: value}, nil
}

func TargetDigestOf(value []byte) TargetDigest {
	digest := DigestBytes(value)
	return TargetDigest{value: digest}
}

func (digest TargetDigest) String() string {
	return digest.value.String()
}

func (digest TargetDigest) equalBytes(value []byte) bool {
	observed := TargetDigestOf(value)
	return digest.value.Equal(observed.value)
}

func (digest TargetDigest) Equal(other TargetDigest) bool {
	return digest.value.Equal(other.value)
}

func (digest TargetDigest) valid() bool {
	return digest.value.valid()
}

type FragmentDigest struct {
	value SHA256
}

func NewFragmentDigest(raw string) (FragmentDigest, error) {
	value, err := NewSHA256(raw)
	if err != nil {
		return FragmentDigest{}, err
	}
	return FragmentDigest{value: value}, nil
}

func FragmentDigestOf(value []byte) FragmentDigest {
	digest := DigestBytes(value)
	return FragmentDigest{value: digest}
}

func (digest FragmentDigest) String() string {
	return digest.value.String()
}

func (digest FragmentDigest) equalBytes(value []byte) bool {
	observed := FragmentDigestOf(value)
	return digest.value.Equal(observed.value)
}

func (digest FragmentDigest) valid() bool {
	return digest.value.valid()
}

type OutsideCarrierDigest struct {
	value SHA256
}

func NewOutsideCarrierDigest(raw string) (OutsideCarrierDigest, error) {
	value, err := NewSHA256(raw)
	if err != nil {
		return OutsideCarrierDigest{}, err
	}
	return OutsideCarrierDigest{value: value}, nil
}

func OutsideCarrierDigestOf(value []byte) OutsideCarrierDigest {
	digest := DigestBytes(value)
	return OutsideCarrierDigest{value: digest}
}

func (digest OutsideCarrierDigest) String() string {
	return digest.value.String()
}

func (digest OutsideCarrierDigest) equalBytes(value []byte) bool {
	observed := OutsideCarrierDigestOf(value)
	return digest.value.Equal(observed.value)
}

func (digest OutsideCarrierDigest) valid() bool {
	return digest.value.valid()
}

func parseOpaqueID(name, raw string) (string, error) {
	if raw != strings.TrimSpace(raw) || !opaqueIDPattern.MatchString(raw) {
		return "", fmt.Errorf("%s %q must be an opaque 1-256 character identifier", name, raw)
	}
	return raw, nil
}

func parseCarrierID(name, raw string) (string, error) {
	if !validCarrierID(raw) {
		return "", fmt.Errorf("%s %q must be a normalized project-relative path", name, raw)
	}
	return raw, nil
}

func validCarrierID(value string) bool {
	if value == "" || value != strings.TrimSpace(value) {
		return false
	}
	if strings.ContainsFunc(value, unicode.IsControl) {
		return false
	}
	if strings.HasPrefix(value, "/") {
		return false
	}
	if strings.Contains(value, `\`) {
		return false
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned != value {
		return false
	}
	return !strings.HasPrefix(cleaned, "../")
}
