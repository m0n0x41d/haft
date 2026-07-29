package cli

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

const initProfileApplicationAuthority = "canonical_project_profile_required_capabilities"

type initProfileApplication struct {
	Authority            string
	ProfileApplicability publicProjectSpecificationApplicability
	RequiredSpecKinds    []project.SpecDocumentKind
	SWEMethodPackKind    projectprofile.CapabilityApplicabilityKind
}

func (application initProfileApplication) valid() bool {
	if application.Authority != initProfileApplicationAuthority {
		return false
	}
	if application.ProfileApplicability.Authority !=
		projectSpecificationReadOnlyAuthority {
		return false
	}
	switch application.ProfileApplicability.Kind {
	case string(projectSpecificationProfileUnderdetermined),
		string(projectSpecificationScopeChoiceRequired),
		string(projectSpecificationRequestedScopeNotFound):
		return len(application.RequiredSpecKinds) == 0 &&
			application.SWEMethodPackKind == "" &&
			application.ProfileApplicability.Cue != nil
	case string(projectSpecificationApplicabilityResolved):
		if application.ProfileApplicability.Cue != nil ||
			!specDocumentKindsEqualStrings(
				application.RequiredSpecKinds,
				application.ProfileApplicability.ApplicableDocumentKinds,
			) {
			return false
		}
	default:
		return false
	}
	return application.SWEMethodPackKind == projectprofile.CapabilityRequired ||
		application.SWEMethodPackKind == projectprofile.CapabilityNotApplicable ||
		application.SWEMethodPackKind == projectprofile.CapabilityUnderdetermined
}

type initProfileSelectionError struct {
	kind              projectSpecificationApplicabilityResolutionKind
	requestedScopeID  string
	availableScopeIDs []string
}

func (failure initProfileSelectionError) Error() string {
	switch failure.kind {
	case projectSpecificationScopeChoiceRequired:
		return fmt.Sprintf(
			"project profile has multiple realization scopes; rerun haft init with one exact --scope-id (%s)",
			strings.Join(failure.availableScopeIDs, ", "),
		)
	case projectSpecificationRequestedScopeNotFound:
		return fmt.Sprintf(
			"requested ScopeID %q is not in the canonical project profile; available ScopeIDs: %s",
			failure.requestedScopeID,
			strings.Join(failure.availableScopeIDs, ", "),
		)
	default:
		return "project profile scope selection failed"
	}
}

func initProfileSelectionFailure(
	application initProfileApplication,
) (initProfileSelectionError, bool) {
	if !application.valid() {
		return initProfileSelectionError{}, false
	}
	applicability := application.ProfileApplicability
	kind := projectSpecificationApplicabilityResolutionKind(
		applicability.Kind,
	)
	if kind != projectSpecificationScopeChoiceRequired &&
		kind != projectSpecificationRequestedScopeNotFound {
		return initProfileSelectionError{}, false
	}
	return initProfileSelectionError{
		kind:             kind,
		requestedScopeID: applicability.RequestedScopeID,
		availableScopeIDs: append(
			[]string{},
			applicability.AvailableScopeIDs...,
		),
	}, true
}

func specDocumentKindsEqualStrings(
	kinds []project.SpecDocumentKind,
	values []string,
) bool {
	if len(kinds) != len(values) {
		return false
	}
	if len(kinds) == 0 {
		return true
	}
	if string(kinds[0]) != values[0] {
		return false
	}
	return specDocumentKindsEqualStrings(kinds[1:], values[1:])
}
