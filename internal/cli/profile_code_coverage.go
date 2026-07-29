package cli

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func profileCodeCoverageRequired(
	readiness canonicalProjectReadiness,
) (bool, error) {
	applicability, resolved := readiness.resolvedApplicability()
	if !resolved {
		return false, nil
	}
	codeApplicability, err := applicability.ScopedCapabilityApplicability(
		projectprofile.CodeDoctrineAndIndexCapability,
	)
	if err != nil {
		return false, err
	}
	return codeApplicability.Kind() == projectprofile.CapabilityRequired, nil
}

func profileAwareCoverageResponse(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
	readiness canonicalProjectReadiness,
	limit int,
) (string, error) {
	applicability, resolved := readiness.resolvedApplicability()
	if !resolved {
		return "## Module coverage\n\n" +
			readiness.profileCue() +
			" Code coverage was not evaluated.", nil
	}
	codeApplicability, err := applicability.ScopedCapabilityApplicability(
		projectprofile.CodeDoctrineAndIndexCapability,
	)
	if err != nil {
		return "", err
	}
	switch codeApplicability.Kind() {
	case projectprofile.CapabilityRequired:
		report, err := codebase.ComputeCoverageWithFileGaps(
			ctx,
			store.DB(),
			projectRoot,
			limit,
		)
		if err != nil {
			return "", fmt.Errorf("compute coverage: %w", err)
		}
		return codebase.FormatCoverageResponse(report), nil
	case projectprofile.CapabilityNotApplicable:
		return fmt.Sprintf(
			"## Module coverage\n\nCode doctrine and index coverage are not applicable in exact project-profile scope %q (profile_payload_digest=%s). No code index or SWE coverage debt was inferred.",
			codeApplicability.ScopeID().String(),
			codeApplicability.ProfilePayloadDigest().String(),
		), nil
	case projectprofile.CapabilityUnderdetermined:
		missingBasis, _ := codeApplicability.MissingBasis()
		return fmt.Sprintf(
			"## Module coverage\n\nCode doctrine and index coverage are underdetermined in exact project-profile scope %q (missing_basis=%s). Code coverage was not evaluated.",
			codeApplicability.ScopeID().String(),
			missingBasis,
		), nil
	default:
		return "", fmt.Errorf(
			"unknown code coverage applicability %q",
			codeApplicability.Kind(),
		)
	}
}
