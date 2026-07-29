package present

import (
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
)

// UserFacingArtifactKindLabel renders an internal artifact kind as plain language.
func UserFacingArtifactKindLabel(kind string) string {
	return artifact.Kind(strings.TrimSpace(kind)).UserFacingLabel()
}

// UserFacingArtifactKindHeading renders an artifact kind as a list heading.
func UserFacingArtifactKindHeading(kind string, count int) string {
	return artifact.Kind(strings.TrimSpace(kind)).UserFacingHeading(count)
}
