package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

// canonicalProjectReadiness keeps filesystem readiness and canonical profile
// applicability separate. A profile cue can therefore suppress false
// software pressure without pretending that an underdetermined profile is a
// ready project.
type canonicalProjectReadiness struct {
	facts              project.ReadinessFacts
	profileEvaluated   bool
	resolution         projectSpecificationApplicabilityResolution
	profileUnavailable bool
}

func inspectCanonicalProjectReadiness(
	ctx context.Context,
	projectRoot string,
	request projectSpecificationScopeRequest,
) (canonicalProjectReadiness, error) {
	facts, err := project.InspectReadiness(projectRoot)
	if err != nil {
		return canonicalProjectReadiness{}, err
	}
	result := canonicalProjectReadiness{
		facts: facts,
	}
	if !facts.HasHaft {
		return result, nil
	}
	resolution, err := resolveCanonicalProjectSpecificationApplicability(
		ctx,
		projectRoot,
		request,
	)
	if err != nil {
		result.profileEvaluated = true
		result.profileUnavailable = true
		result.facts = project.ReadinessFacts{
			Status:  project.ReadinessNeedsOnboard,
			Exists:  true,
			HasHaft: true,
		}
		return result, nil
	}
	result.profileEvaluated = true
	result.resolution = resolution
	applicability, _, resolved := resolution.Resolved()
	if !resolved {
		result.facts = project.ReadinessFacts{
			Status:  project.ReadinessNeedsOnboard,
			Exists:  true,
			HasHaft: true,
		}
		return result, nil
	}
	facts, err = project.InspectReadinessForScope(
		resolution.ProjectRoot().String(),
		applicability,
	)
	if err != nil {
		return canonicalProjectReadiness{}, err
	}
	result.facts = facts
	return result, nil
}

func (readiness canonicalProjectReadiness) resolvedApplicability() (
	project.ProjectSpecificationSetApplicability,
	bool,
) {
	if !readiness.profileEvaluated {
		return project.ProjectSpecificationSetApplicability{}, false
	}
	if readiness.profileUnavailable {
		return project.ProjectSpecificationSetApplicability{}, false
	}
	applicability, _, resolved := readiness.resolution.Resolved()
	return applicability, resolved
}

func (readiness canonicalProjectReadiness) profileCue() string {
	if !readiness.profileEvaluated {
		return ""
	}
	if readiness.profileUnavailable {
		return "Canonical project-profile applicability is unavailable; no software applicability was inferred."
	}
	switch readiness.resolution.Kind() {
	case projectSpecificationApplicabilityResolved:
		return ""
	case projectSpecificationProfileUnderdetermined:
		missingBasis, _ := readiness.resolution.MissingBasis()
		return fmt.Sprintf(
			"Project profile is underdetermined (missing_basis=%s); no software applicability was inferred.",
			missingBasis,
		)
	case projectSpecificationScopeChoiceRequired:
		return fmt.Sprintf(
			"Project profile has several scopes (%s); select one exact scope_id.",
			strings.Join(
				scopeIDStrings(readiness.resolution.AvailableScopeIDs()),
				", ",
			),
		)
	case projectSpecificationRequestedScopeNotFound:
		return fmt.Sprintf(
			"Requested project scope %q is absent; available scope_ids: %s.",
			readiness.resolution.request.scopeID.String(),
			strings.Join(
				scopeIDStrings(readiness.resolution.AvailableScopeIDs()),
				", ",
			),
		)
	default:
		return "Project profile applicability is unavailable."
	}
}

func (readiness canonicalProjectReadiness) fullProfileBasisLine() string {
	applicability, basis, resolved := readiness.resolution.Resolved()
	if !readiness.profileEvaluated || !resolved {
		return ""
	}
	return fmt.Sprintf(
		"Project profile: scope_id=%s; admission_record_ref=%s; admission_record_digest=%s; profile_payload_digest=%s; ledger_revision=%d.",
		applicability.ScopeID().String(),
		basis.admissionRecordRef.String(),
		basis.admissionRecordDigest.String(),
		basis.payloadDigest.String(),
		basis.ledgerRevision.Value(),
	)
}

func (readiness canonicalProjectReadiness) fullProfileCapabilityLines() []string {
	applicability, _, resolved := readiness.resolution.Resolved()
	if !readiness.profileEvaluated || !resolved {
		return nil
	}
	lines := make([]string, 0, len(projectprofile.KnownCapabilities()))
	for _, capability := range projectprofile.KnownCapabilities() {
		entry, err := applicability.ScopedCapabilityApplicability(capability)
		if err != nil {
			return []string{fmt.Sprintf(
				"- applicability_projection=unavailable; capability=%s; detail=%s.",
				capability,
				err,
			)}
		}
		line := fmt.Sprintf(
			"- scope_id=%s; capability=%s; applicability=%s; profile_payload_digest=%s",
			entry.ScopeID().String(),
			entry.Capability(),
			entry.Kind(),
			entry.ProfilePayloadDigest().String(),
		)
		if missingBasis, present := entry.MissingBasis(); present {
			line += "; missing_basis=" + string(missingBasis)
		}
		lines = append(lines, line+".")
	}
	return lines
}

func statusProfilePrefix(
	readiness canonicalProjectReadiness,
	full bool,
) string {
	if cue := readiness.profileCue(); cue != "" {
		return "## Project profile\n\n" +
			cue + "\n\n"
	}
	if !full {
		return ""
	}
	basis := readiness.fullProfileBasisLine()
	if basis == "" {
		return ""
	}
	lines := []string{
		"## Project profile",
		"",
		basis,
		"",
		"Capability applicability (authority=canonical_profile_capability_matrix.v1):",
	}
	lines = append(lines, readiness.fullProfileCapabilityLines()...)
	return strings.Join(lines, "\n") + "\n\n"
}
