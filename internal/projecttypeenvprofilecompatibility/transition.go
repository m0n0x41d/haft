package projecttypeenvprofilecompatibility

import (
	"bytes"
	"fmt"

	typeenvcompatibility "github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvtransitioncompatibility"
)

// The immutable envelope lives below the read-side profile package so the
// TypeEnv selection core can bind it without importing neighborhood runtime.
// This alias keeps the producer API explicit while preserving that layering.
type TransitionProjectionProfileCompatibilitySet = projecttypeenvtransitioncompatibility.Set

type TransitionProjectionProfileCompatibilitySetRef = projecttypeenvtransitioncompatibility.Ref

func ParseTransitionProjectionProfileCompatibilitySetRef(
	raw string,
) (TransitionProjectionProfileCompatibilitySetRef, error) {
	return projecttypeenvtransitioncompatibility.ParseRef(raw)
}

func AssessTransitionProjectionProfiles(
	diff typeenvcompatibility.SuccessorDiff,
) (TransitionProjectionProfileCompatibilitySet, error) {
	profiles, err := AssessInstalledProjectionProfiles(diff)
	if err != nil {
		return TransitionProjectionProfileCompatibilitySet{}, err
	}
	return NewTransitionProjectionProfileCompatibilitySet(diff, profiles)
}

func NewTransitionProjectionProfileCompatibilitySet(
	diff typeenvcompatibility.SuccessorDiff,
	profiles ProjectionProfileCompatibilitySet,
) (TransitionProjectionProfileCompatibilitySet, error) {
	if err := profiles.Verify(); err != nil {
		return TransitionProjectionProfileCompatibilitySet{}, err
	}
	if profiles.BaseTypeEnv() != diff.Base() ||
		profiles.TargetTypeEnv() != diff.Target() ||
		profiles.SuccessorDiffDigest() != diff.Digest() {
		return TransitionProjectionProfileCompatibilitySet{}, fmt.Errorf(
			"transition projection-profile compatibility uses another successor diff",
		)
	}
	return projecttypeenvtransitioncompatibility.New(
		diff,
		profiles.CanonicalBytes(),
	)
}

func DecodeTransitionProjectionProfileCompatibilitySet(
	canonical []byte,
) (TransitionProjectionProfileCompatibilitySet, error) {
	value, err := projecttypeenvtransitioncompatibility.Decode(canonical)
	if err != nil {
		return TransitionProjectionProfileCompatibilitySet{}, err
	}
	profiles, err := DecodeTransitionProjectionProfiles(value)
	if err != nil {
		return TransitionProjectionProfileCompatibilitySet{}, err
	}
	diff := value.SuccessorDiff()
	if profiles.BaseTypeEnv() != diff.Base() ||
		profiles.TargetTypeEnv() != diff.Target() ||
		profiles.SuccessorDiffDigest() != diff.Digest() {
		return TransitionProjectionProfileCompatibilitySet{}, fmt.Errorf(
			"transition projection-profile compatibility uses another successor diff",
		)
	}
	if !bytes.Equal(value.CanonicalBytes(), canonical) {
		return TransitionProjectionProfileCompatibilitySet{}, fmt.Errorf(
			"transition projection-profile compatibility is not canonical",
		)
	}
	return value, nil
}

func DecodeTransitionProjectionProfiles(
	value TransitionProjectionProfileCompatibilitySet,
) (ProjectionProfileCompatibilitySet, error) {
	if err := value.Verify(); err != nil {
		return ProjectionProfileCompatibilitySet{}, err
	}
	profiles, err := DecodeProjectionProfileCompatibilitySet(
		value.ProjectionProfilesCanonicalBytes(),
	)
	if err != nil {
		return ProjectionProfileCompatibilitySet{}, err
	}
	if profiles.Digest() != value.ProjectionProfilesDigest() {
		return ProjectionProfileCompatibilitySet{}, fmt.Errorf(
			"installed projection-profile carrier digest mismatch",
		)
	}
	return profiles, nil
}

func TransitionProjectionProfilesHaveBlockedProfile(
	value TransitionProjectionProfileCompatibilitySet,
) (bool, error) {
	profiles, err := DecodeTransitionProjectionProfiles(value)
	if err != nil {
		return false, err
	}
	return profiles.HasBlockedProfile(), nil
}
