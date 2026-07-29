package method

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

// ScopedCatalogResult is a closed MethodPack catalog projection for one exact
// project-profile ScopeID. A catalog report exists only for Required. Normal
// NotApplicable and Underdetermined results carry no synthetic empty catalog.
type ScopedCatalogResult struct {
	applicability projectprofile.ScopedCapabilityApplicability
	report        CatalogReport
}

func (result ScopedCatalogResult) Applicability() projectprofile.ScopedCapabilityApplicability {
	return result.applicability
}

func (result ScopedCatalogResult) Report() (CatalogReport, bool) {
	if !result.applicability.Valid() ||
		result.applicability.Capability() != projectprofile.SWEMethodPackCapability ||
		result.applicability.Kind() != projectprofile.CapabilityRequired ||
		result.report.Kind != CatalogReportKind ||
		result.report.SchemaVersion != CatalogReportSchemaVersion {
		return CatalogReport{}, false
	}
	return result.report, true
}

// ScopedPullResult is the pure MethodPack pull result for one exact project
// scope. NotApplicable and Underdetermined do not manufacture a MethodRun.
type ScopedPullResult struct {
	applicability projectprofile.ScopedCapabilityApplicability
	run           MethodRun
}

func (result ScopedPullResult) Applicability() projectprofile.ScopedCapabilityApplicability {
	return result.applicability
}

func (result ScopedPullResult) Run() (MethodRun, bool) {
	if !result.applicability.Valid() ||
		result.applicability.Capability() != projectprofile.SWEMethodPackCapability ||
		result.applicability.Kind() != projectprofile.CapabilityRequired ||
		result.run.CatalogID == "" ||
		result.run.Status == "" {
		return MethodRun{}, false
	}
	return result.run, true
}

// DiscoverCatalogForScope exposes the built-in SWE catalog only when the
// central profile matrix marks swe_methodpack Required for the exact ScopeID.
// It performs no profile lookup and no automatic scope selection.
func DiscoverCatalogForScope(
	matrix projectprofile.CapabilityApplicabilityMatrix,
	scopeID projectprofile.ScopeID,
	status string,
) (ScopedCatalogResult, error) {
	applicability, err := projectprofile.ResolveScopedCapabilityApplicability(
		matrix,
		scopeID,
		projectprofile.SWEMethodPackCapability,
	)
	if err != nil {
		return ScopedCatalogResult{}, err
	}
	result := ScopedCatalogResult{applicability: applicability}
	if applicability.Kind() != projectprofile.CapabilityRequired {
		return result, nil
	}
	report, err := DiscoverCatalog(status)
	if err != nil {
		return ScopedCatalogResult{}, err
	}
	result.report = report
	if _, present := result.Report(); !present {
		return ScopedCatalogResult{}, fmt.Errorf(
			"required scoped MethodPack catalog result is invalid",
		)
	}
	return result, nil
}

// PullForScope applies ordinary task matching only after exact scope-local
// MethodPack applicability has resolved Required. It creates no artifact and
// performs no profile or filesystem IO.
func PullForScope(
	matrix projectprofile.CapabilityApplicabilityMatrix,
	scopeID projectprofile.ScopeID,
	input PullInput,
) (ScopedPullResult, error) {
	applicability, err := projectprofile.ResolveScopedCapabilityApplicability(
		matrix,
		scopeID,
		projectprofile.SWEMethodPackCapability,
	)
	if err != nil {
		return ScopedPullResult{}, err
	}
	result := ScopedPullResult{applicability: applicability}
	if applicability.Kind() != projectprofile.CapabilityRequired {
		return result, nil
	}
	run, err := Pull(input)
	if err != nil {
		return ScopedPullResult{}, err
	}
	result.run = run
	if _, present := result.Run(); !present {
		return ScopedPullResult{}, fmt.Errorf(
			"required scoped MethodPack pull result is invalid",
		)
	}
	return result, nil
}
