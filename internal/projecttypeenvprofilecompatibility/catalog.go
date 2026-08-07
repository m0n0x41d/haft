package projecttypeenvprofilecompatibility

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectmemory/projectionprofile"
	typeenvcompatibility "github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const profileCompatibilitySetCanonicalDomain = "haft.projection-profile-typeenv-compatibility-set.v1"

// ProjectionProfileCompatibilitySet proves that every installed immutable
// ProjectionProfile edition was assessed against one exact successor diff.
// Its order is canonical identity order only; it is not priority or an
// execution order.
type ProjectionProfileCompatibilitySet struct {
	base       typedmemory.TypeEnvRef
	target     typedmemory.TypeEnvRef
	diffDigest typedmemory.SHA256Digest
	profiles   []ProjectionProfileCompatibility
	digest     typedmemory.SHA256Digest
}

func AssessInstalledProjectionProfiles(
	diff typeenvcompatibility.SuccessorDiff,
) (ProjectionProfileCompatibilitySet, error) {
	if err := diff.Verify(); err != nil {
		return ProjectionProfileCompatibilitySet{}, fmt.Errorf("successor diff: %w", err)
	}
	profiles := projectionprofile.Installed()
	results := make([]ProjectionProfileCompatibility, 0, len(profiles))
	for _, profile := range profiles {
		result, err := AssessProjectionProfile(profile, diff)
		if err != nil {
			return ProjectionProfileCompatibilitySet{}, fmt.Errorf(
				"assess projection profile %s: %w",
				profile.Ref().String(),
				err,
			)
		}
		results = append(results, result)
	}
	result := ProjectionProfileCompatibilitySet{
		base:       diff.Base(),
		target:     diff.Target(),
		diffDigest: diff.Digest(),
		profiles:   canonicalProfileCompatibilities(results),
	}
	digest, err := digestBytes(result.CanonicalBytes())
	if err != nil {
		return ProjectionProfileCompatibilitySet{}, err
	}
	result.digest = digest
	if err := result.Verify(); err != nil {
		return ProjectionProfileCompatibilitySet{}, err
	}
	return result, nil
}

func (set ProjectionProfileCompatibilitySet) BaseTypeEnv() typedmemory.TypeEnvRef {
	return set.base
}

func (set ProjectionProfileCompatibilitySet) TargetTypeEnv() typedmemory.TypeEnvRef {
	return set.target
}

func (set ProjectionProfileCompatibilitySet) SuccessorDiffDigest() typedmemory.SHA256Digest {
	return set.diffDigest
}

func (set ProjectionProfileCompatibilitySet) Profiles() []ProjectionProfileCompatibility {
	return append([]ProjectionProfileCompatibility(nil), set.profiles...)
}

func (set ProjectionProfileCompatibilitySet) Digest() typedmemory.SHA256Digest {
	return set.digest
}

func (set ProjectionProfileCompatibilitySet) HasBlockedProfile() bool {
	for _, profile := range set.profiles {
		if profile.Kind() == ProfileBlocked {
			return true
		}
	}
	return false
}

func (set ProjectionProfileCompatibilitySet) CanonicalBytes() []byte {
	writer := newCanonicalWriter(profileCompatibilitySetCanonicalDomain)
	writer.addString(set.base.String())
	writer.addString(set.target.String())
	writer.addString(set.diffDigest.String())
	writer.addUint64(uint64(len(set.profiles)))
	for _, profile := range set.profiles {
		writer.addBytes(profile.CanonicalBytes())
	}
	return writer.bytes()
}

func (set ProjectionProfileCompatibilitySet) Verify() error {
	if _, err := typedmemory.ParseTypeEnvRef(set.base.String()); err != nil {
		return fmt.Errorf("profile compatibility set base is invalid")
	}
	if _, err := typedmemory.ParseTypeEnvRef(set.target.String()); err != nil {
		return fmt.Errorf("profile compatibility set target is invalid")
	}
	if _, err := typedmemory.NewSHA256Digest(set.diffDigest.String()); err != nil {
		return fmt.Errorf("profile compatibility set diff digest is invalid")
	}
	installed := projectionprofile.Installed()
	if len(set.profiles) != len(installed) {
		return fmt.Errorf("profile compatibility set does not cover every installed edition")
	}
	for index, profile := range set.profiles {
		if profile == nil {
			return fmt.Errorf("profile compatibility set contains a nil result")
		}
		if err := profile.Verify(); err != nil {
			return fmt.Errorf("profile compatibility result %d: %w", index, err)
		}
		if profile.BaseTypeEnv() != set.base ||
			profile.TargetTypeEnv() != set.target ||
			profile.SuccessorDiffDigest() != set.diffDigest {
			return fmt.Errorf("profile compatibility result uses another successor basis")
		}
		expected := installed[index]
		if profile.ProfileRef() != expected.Ref() ||
			profile.ProfileEdition() != expected.Edition() ||
			profile.ProfileDigest() != expected.Digest() {
			return fmt.Errorf("profile compatibility set differs from installed profile catalog")
		}
		if index > 0 && profileCompatibilityIdentity(set.profiles[index-1]) >= profileCompatibilityIdentity(profile) {
			return fmt.Errorf("profile compatibility set is not canonical")
		}
	}
	digest, err := digestBytes(set.CanonicalBytes())
	if err != nil {
		return err
	}
	if digest != set.digest {
		return fmt.Errorf("profile compatibility set digest mismatch")
	}
	return nil
}

func DecodeProjectionProfileCompatibilitySet(
	canonical []byte,
) (ProjectionProfileCompatibilitySet, error) {
	if len(canonical) == 0 || len(canonical) > maximumProfileCompatibilityBytes {
		return ProjectionProfileCompatibilitySet{}, fmt.Errorf("profile compatibility set byte length is invalid")
	}
	reader := newCanonicalReader(canonical)
	domain, err := reader.readString("profile compatibility set domain")
	if err != nil {
		return ProjectionProfileCompatibilitySet{}, err
	}
	if domain != profileCompatibilitySetCanonicalDomain {
		return ProjectionProfileCompatibilitySet{}, fmt.Errorf("profile compatibility set domain is invalid")
	}
	baseText, err := reader.readString("profile compatibility set base")
	if err != nil {
		return ProjectionProfileCompatibilitySet{}, err
	}
	base, err := typedmemory.ParseTypeEnvRef(baseText)
	if err != nil {
		return ProjectionProfileCompatibilitySet{}, err
	}
	targetText, err := reader.readString("profile compatibility set target")
	if err != nil {
		return ProjectionProfileCompatibilitySet{}, err
	}
	target, err := typedmemory.ParseTypeEnvRef(targetText)
	if err != nil {
		return ProjectionProfileCompatibilitySet{}, err
	}
	diffDigest, err := reader.readDigest("profile compatibility set diff digest")
	if err != nil {
		return ProjectionProfileCompatibilitySet{}, err
	}
	installed := projectionprofile.Installed()
	profileCount, err := reader.readCount(
		"profile compatibility set profiles",
		len(installed),
	)
	if err != nil || profileCount != len(installed) {
		return ProjectionProfileCompatibilitySet{}, fmt.Errorf(
			"profile compatibility set profile count is invalid",
		)
	}
	profiles := make([]ProjectionProfileCompatibility, 0, profileCount)
	for index := 0; index < profileCount; index++ {
		raw, readErr := reader.readBytes("profile compatibility set result")
		if readErr != nil {
			return ProjectionProfileCompatibilitySet{}, readErr
		}
		profile, decodeErr := DecodeProjectionProfileCompatibility(raw)
		if decodeErr != nil {
			return ProjectionProfileCompatibilitySet{}, decodeErr
		}
		profiles = append(profiles, profile)
	}
	if reader.remaining() != 0 {
		return ProjectionProfileCompatibilitySet{}, fmt.Errorf("profile compatibility set has trailing bytes")
	}
	set := ProjectionProfileCompatibilitySet{
		base:       base,
		target:     target,
		diffDigest: diffDigest,
		profiles:   profiles,
	}
	digest, err := digestBytes(set.CanonicalBytes())
	if err != nil {
		return ProjectionProfileCompatibilitySet{}, err
	}
	set.digest = digest
	if !bytes.Equal(set.CanonicalBytes(), canonical) {
		return ProjectionProfileCompatibilitySet{}, fmt.Errorf("profile compatibility set is not canonical")
	}
	if err := set.Verify(); err != nil {
		return ProjectionProfileCompatibilitySet{}, err
	}
	return set, nil
}

func canonicalProfileCompatibilities(
	values []ProjectionProfileCompatibility,
) []ProjectionProfileCompatibility {
	result := append([]ProjectionProfileCompatibility(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		return profileCompatibilityIdentity(result[left]) < profileCompatibilityIdentity(result[right])
	})
	return result
}

func profileCompatibilityIdentity(value ProjectionProfileCompatibility) string {
	return value.ProfileRef().String() + "\x00" +
		value.ProfileDigest().String() + "\x00" +
		fmt.Sprintf("%010d", value.ProfileEdition())
}
