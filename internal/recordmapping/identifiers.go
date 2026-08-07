// Package recordmapping owns the lower-layer strong identities shared by
// record carriers and registration policy. It has no project-memory, TypeEnv,
// persistence, activation, or executable-runtime dependency.
package recordmapping

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

var (
	exactSemverPattern = regexp.MustCompile(
		`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`,
	)
	exactAdapterNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]*$`)
)

type MappingManifestRef struct {
	id      string
	version string
	digest  typedmemory.SHA256Digest
}

func NewMappingManifestRef(
	id string,
	version string,
	digest typedmemory.SHA256Digest,
) (MappingManifestRef, error) {
	parsedID, err := parseExactText("mapping-manifest ID", id)
	if err != nil {
		return MappingManifestRef{}, err
	}
	parsedVersion, err := parseExactText("mapping-manifest version", version)
	if err != nil {
		return MappingManifestRef{}, err
	}
	if !isExactSemver(parsedVersion) {
		return MappingManifestRef{}, fmt.Errorf(
			"mapping-manifest version must be an exact semantic version, got %q",
			parsedVersion,
		)
	}
	parsedDigest, err := typedmemory.NewSHA256Digest(digest.String())
	if err != nil || parsedDigest != digest {
		return MappingManifestRef{}, fmt.Errorf("mapping-manifest digest is invalid")
	}
	return MappingManifestRef{
		id:      parsedID,
		version: parsedVersion,
		digest:  parsedDigest,
	}, nil
}

func ParseMappingManifestRef(raw string) (MappingManifestRef, error) {
	body, found := strings.CutPrefix(raw, "mapping-manifest:")
	if !found {
		return MappingManifestRef{}, fmt.Errorf("mapping-manifest reference is malformed")
	}
	id, remainder, err := parseLengthPrefixedSegment(body)
	if err != nil {
		return MappingManifestRef{}, fmt.Errorf("mapping-manifest reference ID: %w", err)
	}
	version, digestRaw, err := parseLengthPrefixedSegment(remainder)
	if err != nil {
		return MappingManifestRef{}, fmt.Errorf("mapping-manifest reference version: %w", err)
	}
	digest, err := typedmemory.NewSHA256Digest(digestRaw)
	if err != nil {
		return MappingManifestRef{}, fmt.Errorf("mapping-manifest reference digest: %w", err)
	}
	ref, err := NewMappingManifestRef(id, version, digest)
	if err != nil {
		return MappingManifestRef{}, err
	}
	if ref.String() != raw {
		return MappingManifestRef{}, fmt.Errorf("mapping-manifest reference is not canonical")
	}
	return ref, nil
}

func (ref MappingManifestRef) ID() string { return ref.id }

func (ref MappingManifestRef) Version() string { return ref.version }

func (ref MappingManifestRef) Digest() typedmemory.SHA256Digest { return ref.digest }

func (ref MappingManifestRef) String() string {
	return fmt.Sprintf(
		"mapping-manifest:%d:%s%d:%s%s",
		len(ref.id),
		ref.id,
		len(ref.version),
		ref.version,
		ref.digest.String(),
	)
}

func (ref MappingManifestRef) Verify() error {
	parsed, err := ParseMappingManifestRef(ref.String())
	if err != nil || parsed != ref {
		return fmt.Errorf("mapping-manifest reference is invalid")
	}
	return nil
}

type AdapterVersion struct {
	value string
}

func NewAdapterVersion(raw string) (AdapterVersion, error) {
	value, err := parseExactText("record adapter version", raw)
	if err != nil {
		return AdapterVersion{}, err
	}
	if !isExactAdapterVersion(value) {
		return AdapterVersion{}, fmt.Errorf(
			"record adapter version must be an exact semantic version coordinate, got %q",
			value,
		)
	}
	return AdapterVersion{value: value}, nil
}

func (version AdapterVersion) String() string { return version.value }

func (version AdapterVersion) Verify() error {
	parsed, err := NewAdapterVersion(version.value)
	if err != nil || parsed != version {
		return fmt.Errorf("record adapter version is invalid")
	}
	return nil
}

func parseExactText(label string, raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	if !utf8.ValidString(raw) {
		return "", fmt.Errorf("%s must be valid UTF-8", label)
	}
	if raw != strings.TrimSpace(raw) {
		return "", fmt.Errorf("%s must not have surrounding whitespace", label)
	}
	for _, current := range raw {
		if unicode.IsControl(current) {
			return "", fmt.Errorf("%s must not contain control characters", label)
		}
	}
	return raw, nil
}

func isExactAdapterVersion(raw string) bool {
	name, version, found := strings.Cut(raw, "/")
	if !found {
		return isExactSemver(raw)
	}
	if strings.Contains(version, "/") || !exactAdapterNamePattern.MatchString(name) {
		return false
	}
	return isExactSemver(version)
}

func isExactSemver(raw string) bool {
	parts := exactSemverPattern.FindStringSubmatch(raw)
	if parts == nil {
		return false
	}
	prerelease := parts[4]
	if prerelease == "" {
		return true
	}
	for _, identifier := range strings.Split(prerelease, ".") {
		if numericIdentifierHasLeadingZero(identifier) {
			return false
		}
	}
	return true
}

func numericIdentifierHasLeadingZero(raw string) bool {
	if len(raw) < 2 || raw[0] != '0' {
		return false
	}
	for _, current := range raw {
		if current < '0' || current > '9' {
			return false
		}
	}
	return true
}

func parseLengthPrefixedSegment(raw string) (string, string, error) {
	lengthRaw, remainder, found := strings.Cut(raw, ":")
	if !found || lengthRaw == "" {
		return "", "", fmt.Errorf("length-prefixed segment is malformed")
	}
	length, err := strconv.Atoi(lengthRaw)
	if err != nil || length < 1 {
		return "", "", fmt.Errorf("length-prefixed segment has an invalid length")
	}
	if strconv.Itoa(length) != lengthRaw {
		return "", "", fmt.Errorf("length-prefixed segment length is not canonical")
	}
	if len(remainder) < length {
		return "", "", fmt.Errorf("length-prefixed segment is truncated")
	}
	return remainder[:length], remainder[length:], nil
}
