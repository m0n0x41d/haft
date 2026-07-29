// Package legacyimport defines the pure classification and deterministic
// opaque-history mapping algebra for legacy project-memory import. The outer
// legacyimporteffect package owns the sealed effect boundary; this package
// cannot perform storage writes, semantic admission, or ProjectTypeEnv
// activation.
package legacyimport

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const digestPrefix = "sha256:"

type CarrierFormat struct {
	value string
}

func NewCarrierFormat(raw string) (CarrierFormat, error) {
	value, err := parseExactIdentifier("carrier format", raw)
	if err != nil {
		return CarrierFormat{}, err
	}
	return CarrierFormat{value: value}, nil
}

func (format CarrierFormat) String() string { return format.value }

func (format CarrierFormat) valid() bool { return format.value != "" }

type SourceCoordinate struct {
	value string
}

func NewSourceCoordinate(raw string) (SourceCoordinate, error) {
	value, err := parseExactIdentifier("source coordinate", raw)
	if err != nil {
		return SourceCoordinate{}, err
	}
	return SourceCoordinate{value: value}, nil
}

func (coordinate SourceCoordinate) String() string { return coordinate.value }

func (coordinate SourceCoordinate) valid() bool { return coordinate.value != "" }

type LegacyIdentityRef struct {
	value string
}

func NewLegacyIdentityRef(raw string) (LegacyIdentityRef, error) {
	value, err := parseExactIdentifier("legacy identity reference", raw)
	if err != nil {
		return LegacyIdentityRef{}, err
	}
	return LegacyIdentityRef{value: value}, nil
}

func (ref LegacyIdentityRef) String() string { return ref.value }

func (ref LegacyIdentityRef) valid() bool { return ref.value != "" }

type SemanticSubjectRef struct {
	value string
}

func NewSemanticSubjectRef(raw string) (SemanticSubjectRef, error) {
	value, err := parseExactIdentifier("semantic subject reference", raw)
	if err != nil {
		return SemanticSubjectRef{}, err
	}
	return SemanticSubjectRef{value: value}, nil
}

func (ref SemanticSubjectRef) String() string { return ref.value }

func (ref SemanticSubjectRef) valid() bool { return ref.value != "" }

type AssociationLabel struct {
	value string
}

func NewAssociationLabel(raw string) (AssociationLabel, error) {
	value, err := parseExactIdentifier("legacy association label", raw)
	if err != nil {
		return AssociationLabel{}, err
	}
	return AssociationLabel{value: value}, nil
}

func (label AssociationLabel) String() string { return label.value }

func (label AssociationLabel) valid() bool { return label.value != "" }

type ClassifierVersion struct {
	value string
}

func NewClassifierVersion(raw string) (ClassifierVersion, error) {
	value, err := parseExactIdentifier("classifier version", raw)
	if err != nil {
		return ClassifierVersion{}, err
	}
	return ClassifierVersion{value: value}, nil
}

func (version ClassifierVersion) String() string { return version.value }

func (version ClassifierVersion) valid() bool { return version.value != "" }

type UnresolvedReason struct {
	value string
}

func NewUnresolvedReason(raw string) (UnresolvedReason, error) {
	value, err := parseExactIdentifier("unresolved basis reason", raw)
	if err != nil {
		return UnresolvedReason{}, err
	}
	return UnresolvedReason{value: value}, nil
}

func (reason UnresolvedReason) String() string { return reason.value }

func (reason UnresolvedReason) valid() bool { return reason.value != "" }

func digestBytes(value []byte) typedmemory.SHA256Digest {
	sum := sha256.Sum256(value)
	encoded := hex.EncodeToString(sum[:])
	digest, _ := typedmemory.NewSHA256Digest(digestPrefix + encoded)
	return digest
}

func parseExactIdentifier(label, raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	if raw != strings.TrimSpace(raw) {
		return "", fmt.Errorf("%s must not have surrounding whitespace", label)
	}
	if !utf8.ValidString(raw) {
		return "", fmt.Errorf("%s must be valid UTF-8", label)
	}
	for _, current := range raw {
		if unicode.IsControl(current) {
			return "", fmt.Errorf("%s must not contain control characters", label)
		}
	}
	return raw, nil
}
