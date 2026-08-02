package cli

import (
	"context"
	"fmt"
	"slices"

	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

type projectSpecificationScopeRequestKind string

const (
	projectSpecificationScopeAutomatic projectSpecificationScopeRequestKind = "automatic_single_scope"
	projectSpecificationScopeExact     projectSpecificationScopeRequestKind = "exact_scope"
)

type projectSpecificationScopeRequest struct {
	kind    projectSpecificationScopeRequestKind
	scopeID projectprofile.ScopeID
}

func automaticProjectSpecificationScopeRequest() projectSpecificationScopeRequest {
	return projectSpecificationScopeRequest{
		kind: projectSpecificationScopeAutomatic,
	}
}

func exactProjectSpecificationScopeRequest(
	scopeID projectprofile.ScopeID,
) (projectSpecificationScopeRequest, error) {
	request := projectSpecificationScopeRequest{
		kind:    projectSpecificationScopeExact,
		scopeID: scopeID,
	}
	if !request.valid() {
		return projectSpecificationScopeRequest{}, fmt.Errorf(
			"exact project specification ScopeID is invalid",
		)
	}
	return request, nil
}

func (request projectSpecificationScopeRequest) valid() bool {
	switch request.kind {
	case projectSpecificationScopeAutomatic:
		return request.scopeID.String() == ""
	case projectSpecificationScopeExact:
		scopeID, err := projectprofile.NewScopeID(request.scopeID.String())
		return err == nil && scopeID == request.scopeID
	default:
		return false
	}
}

type projectSpecificationScopeSelectionKind string

const (
	projectSpecificationScopeSelected          projectSpecificationScopeSelectionKind = "selected"
	projectSpecificationScopeSelectionRequired projectSpecificationScopeSelectionKind = "selection_required"
	projectSpecificationScopeNotFound          projectSpecificationScopeSelectionKind = "not_found"
)

type projectSpecificationScopeSelection struct {
	kind      projectSpecificationScopeSelectionKind
	selected  projectprofile.ScopeID
	requested projectprofile.ScopeID
	available []projectprofile.ScopeID
}

func (selection projectSpecificationScopeSelection) valid() bool {
	available := canonicalScopeIDs(selection.available)
	if !slices.Equal(available, selection.available) || len(available) == 0 {
		return false
	}
	switch selection.kind {
	case projectSpecificationScopeSelected:
		return selection.requested.String() == "" &&
			scopeIDValid(selection.selected) &&
			slices.Contains(available, selection.selected)
	case projectSpecificationScopeSelectionRequired:
		return selection.selected.String() == "" &&
			selection.requested.String() == "" &&
			len(available) > 1
	case projectSpecificationScopeNotFound:
		return selection.selected.String() == "" &&
			scopeIDValid(selection.requested) &&
			!slices.Contains(available, selection.requested)
	default:
		return false
	}
}

func (selection projectSpecificationScopeSelection) SelectedScopeID() (
	projectprofile.ScopeID,
	bool,
) {
	if !selection.valid() || selection.kind != projectSpecificationScopeSelected {
		return projectprofile.ScopeID{}, false
	}
	return selection.selected, true
}

func (selection projectSpecificationScopeSelection) AvailableScopeIDs() []projectprofile.ScopeID {
	if !selection.valid() {
		return nil
	}
	return append([]projectprofile.ScopeID{}, selection.available...)
}

func selectProjectSpecificationScope(
	matrix projectprofile.CapabilityApplicabilityMatrix,
	request projectSpecificationScopeRequest,
) (projectSpecificationScopeSelection, error) {
	if !matrix.Valid() {
		return projectSpecificationScopeSelection{}, fmt.Errorf(
			"project capability applicability matrix is invalid",
		)
	}
	if !request.valid() {
		return projectSpecificationScopeSelection{}, fmt.Errorf(
			"project specification scope request is invalid",
		)
	}
	available := canonicalScopeIDs(matrix.ScopeIDs())
	if request.kind == projectSpecificationScopeExact {
		if slices.Contains(available, request.scopeID) {
			return projectSpecificationScopeSelection{
				kind:      projectSpecificationScopeSelected,
				selected:  request.scopeID,
				available: available,
			}, nil
		}
		return projectSpecificationScopeSelection{
			kind:      projectSpecificationScopeNotFound,
			requested: request.scopeID,
			available: available,
		}, nil
	}
	if len(available) == 1 {
		return projectSpecificationScopeSelection{
			kind:      projectSpecificationScopeSelected,
			selected:  available[0],
			available: available,
		}, nil
	}
	return projectSpecificationScopeSelection{
		kind:      projectSpecificationScopeSelectionRequired,
		available: available,
	}, nil
}

type canonicalProfileApplicabilityBasis struct {
	projectRoot           projectprofile.ProjectRootV1
	origin                projectprofile.ProfileAdmissionOrigin
	admissionRecordRef    projectprofile.ProfileDeclarationAdmissionRecordRef
	admissionRecordDigest projectprofile.ContentDigest
	payloadDigest         projectprofile.ContentDigest
	ledgerRevision        projectprofile.LedgerRevision
}

func (basis canonicalProfileApplicabilityBasis) valid() bool {
	return basis.projectRoot.String() != "" &&
		basis.origin != "" &&
		basis.admissionRecordRef.String() != "" &&
		basis.admissionRecordDigest.String() != "" &&
		basis.payloadDigest.String() != "" &&
		basis.ledgerRevision.Value() > 0
}

type projectSpecificationApplicabilityResolutionKind string

const (
	projectSpecificationApplicabilityResolved  projectSpecificationApplicabilityResolutionKind = "resolved"
	projectSpecificationProfileUnderdetermined projectSpecificationApplicabilityResolutionKind = "profile_underdetermined"
	projectSpecificationScopeChoiceRequired    projectSpecificationApplicabilityResolutionKind = "scope_selection_required"
	projectSpecificationRequestedScopeNotFound projectSpecificationApplicabilityResolutionKind = "requested_scope_not_found"
)

type projectSpecificationApplicabilityResolution struct {
	kind            projectSpecificationApplicabilityResolutionKind
	projectRoot     projectprofile.ProjectRootV1
	request         projectSpecificationScopeRequest
	applicability   project.ProjectSpecificationSetApplicability
	basis           canonicalProfileApplicabilityBasis
	missingBasis    profileadmissionsqlite.CapabilityApplicabilityMissingBasis
	availableScopes []projectprofile.ScopeID
}

func (resolution projectSpecificationApplicabilityResolution) Kind() projectSpecificationApplicabilityResolutionKind {
	return resolution.kind
}

func (resolution projectSpecificationApplicabilityResolution) Valid() bool {
	available := canonicalScopeIDs(resolution.availableScopes)
	availableCanonical := slices.Equal(available, resolution.availableScopes)
	switch resolution.kind {
	case projectSpecificationApplicabilityResolved:
		return resolution.projectRoot.String() != "" &&
			resolution.request.valid() &&
			resolution.applicability.Valid() &&
			resolution.basis.valid() &&
			resolution.projectRoot == resolution.basis.projectRoot &&
			resolution.applicability.ProfilePayloadDigest() ==
				resolution.basis.payloadDigest &&
			resolution.missingBasis == "" &&
			len(available) > 0 &&
			availableCanonical &&
			slices.Contains(available, resolution.applicability.ScopeID()) &&
			resolvedScopeMatchesRequest(
				resolution.applicability.ScopeID(),
				available,
				resolution.request,
			)
	case projectSpecificationProfileUnderdetermined:
		return resolution.projectRoot.String() != "" &&
			resolution.request.valid() &&
			!resolution.applicability.Valid() &&
			!resolution.basis.valid() &&
			validRuntimeApplicabilityMissingBasis(resolution.missingBasis) &&
			len(resolution.availableScopes) == 0
	case projectSpecificationScopeChoiceRequired:
		return resolution.projectRoot.String() != "" &&
			resolution.request == automaticProjectSpecificationScopeRequest() &&
			!resolution.applicability.Valid() &&
			resolution.basis.valid() &&
			resolution.projectRoot == resolution.basis.projectRoot &&
			resolution.missingBasis == "" &&
			len(available) > 1 &&
			availableCanonical
	case projectSpecificationRequestedScopeNotFound:
		return resolution.projectRoot.String() != "" &&
			resolution.request.valid() &&
			resolution.request.kind == projectSpecificationScopeExact &&
			!resolution.applicability.Valid() &&
			resolution.basis.valid() &&
			resolution.projectRoot == resolution.basis.projectRoot &&
			resolution.missingBasis == "" &&
			len(available) > 0 &&
			availableCanonical &&
			!slices.Contains(available, resolution.request.scopeID)
	default:
		return false
	}
}

func (resolution projectSpecificationApplicabilityResolution) ProjectRoot() projectprofile.ProjectRootV1 {
	if !resolution.Valid() {
		return projectprofile.ProjectRootV1{}
	}
	return resolution.projectRoot
}

func (resolution projectSpecificationApplicabilityResolution) Resolved() (
	project.ProjectSpecificationSetApplicability,
	canonicalProfileApplicabilityBasis,
	bool,
) {
	if !resolution.Valid() ||
		resolution.kind != projectSpecificationApplicabilityResolved {
		return project.ProjectSpecificationSetApplicability{},
			canonicalProfileApplicabilityBasis{},
			false
	}
	return resolution.applicability, resolution.basis, true
}

func (resolution projectSpecificationApplicabilityResolution) AvailableScopeIDs() []projectprofile.ScopeID {
	if !resolution.Valid() {
		return nil
	}
	return append([]projectprofile.ScopeID{}, resolution.availableScopes...)
}

func (resolution projectSpecificationApplicabilityResolution) MissingBasis() (
	profileadmissionsqlite.CapabilityApplicabilityMissingBasis,
	bool,
) {
	if !resolution.Valid() ||
		resolution.kind != projectSpecificationProfileUnderdetermined {
		return "", false
	}
	return resolution.missingBasis, true
}

func resolveCanonicalProjectSpecificationApplicability(
	ctx context.Context,
	projectRoot string,
	request projectSpecificationScopeRequest,
) (projectSpecificationApplicabilityResolution, error) {
	if ctx == nil {
		return projectSpecificationApplicabilityResolution{}, fmt.Errorf(
			"project specification applicability context is required",
		)
	}
	if !request.valid() {
		return projectSpecificationApplicabilityResolution{}, fmt.Errorf(
			"project specification scope request is invalid",
		)
	}
	handle, err := openCurrentProjectLedger(
		ctx,
		projectRoot,
		projectledger.ReadOnly,
		"project-profile applicability read",
	)
	if err != nil {
		return projectSpecificationApplicabilityResolution{}, fmt.Errorf(
			"open project profile ledger: %w",
			err,
		)
	}
	service, err := profileadmissionsqlite.NewService(handle.Database())
	if err != nil {
		_ = handle.Close()
		return projectSpecificationApplicabilityResolution{}, fmt.Errorf(
			"construct canonical profile applicability resolver: %w",
			err,
		)
	}
	profileRoot, err := projectprofile.NewProjectRootV1(
		handle.ProjectRoot().String(),
	)
	if err != nil {
		_ = handle.Close()
		return projectSpecificationApplicabilityResolution{}, err
	}
	matrixResult := service.ResolveCapabilityApplicabilityMatrix(
		ctx,
		profileRoot,
	)
	resolution, resolveErr := projectSpecificationApplicabilityFromCanonicalResult(
		matrixResult,
		request,
	)
	closeErr := handle.Close()
	if resolveErr != nil {
		return projectSpecificationApplicabilityResolution{}, resolveErr
	}
	if closeErr != nil {
		return projectSpecificationApplicabilityResolution{}, fmt.Errorf(
			"close project profile ledger: %w",
			closeErr,
		)
	}
	return resolution, nil
}

func projectSpecificationApplicabilityFromCanonicalResult(
	result profileadmissionsqlite.CapabilityApplicabilityMatrixResult,
	request projectSpecificationScopeRequest,
) (projectSpecificationApplicabilityResolution, error) {
	if !result.Valid() {
		return projectSpecificationApplicabilityResolution{}, fmt.Errorf(
			"canonical profile applicability result is invalid",
		)
	}
	if underdetermined, ok := result.Underdetermined(); ok {
		resolution := projectSpecificationApplicabilityResolution{
			projectRoot:  underdetermined.ProjectRoot(),
			kind:         projectSpecificationProfileUnderdetermined,
			request:      request,
			missingBasis: underdetermined.MissingBasis(),
		}
		if !resolution.Valid() {
			return projectSpecificationApplicabilityResolution{}, fmt.Errorf(
				"canonical profile underdetermined result is invalid",
			)
		}
		return resolution, nil
	}
	resolved, ok := result.Resolved()
	if !ok {
		return projectSpecificationApplicabilityResolution{}, fmt.Errorf(
			"canonical profile applicability result has no known variant",
		)
	}
	basis := canonicalProfileApplicabilityBasis{
		projectRoot:           resolved.ProjectRoot(),
		origin:                resolved.Origin(),
		admissionRecordRef:    resolved.AdmissionRecordRef(),
		admissionRecordDigest: resolved.AdmissionRecordDigest(),
		payloadDigest:         resolved.ProfilePayloadDigest(),
		ledgerRevision:        resolved.LedgerRevision(),
	}
	return projectSpecificationApplicabilityFromMatrix(
		resolved.Matrix(),
		basis,
		request,
	)
}

func projectSpecificationApplicabilityFromMatrix(
	matrix projectprofile.CapabilityApplicabilityMatrix,
	basis canonicalProfileApplicabilityBasis,
	request projectSpecificationScopeRequest,
) (projectSpecificationApplicabilityResolution, error) {
	if !basis.valid() || basis.payloadDigest != matrix.ProfilePayloadDigest() {
		return projectSpecificationApplicabilityResolution{}, fmt.Errorf(
			"canonical profile applicability basis does not match the matrix",
		)
	}
	selection, err := selectProjectSpecificationScope(matrix, request)
	if err != nil {
		return projectSpecificationApplicabilityResolution{}, err
	}
	available := selection.AvailableScopeIDs()
	if scopeID, selected := selection.SelectedScopeID(); selected {
		applicability, err := project.DeriveProjectSpecificationSetApplicability(
			matrix,
			scopeID,
		)
		if err != nil {
			return projectSpecificationApplicabilityResolution{}, err
		}
		resolution := projectSpecificationApplicabilityResolution{
			kind:            projectSpecificationApplicabilityResolved,
			projectRoot:     basis.projectRoot,
			request:         request,
			applicability:   applicability,
			basis:           basis,
			availableScopes: available,
		}
		if !resolution.Valid() {
			return projectSpecificationApplicabilityResolution{}, fmt.Errorf(
				"resolved project specification applicability is invalid",
			)
		}
		return resolution, nil
	}
	if selection.kind == projectSpecificationScopeSelectionRequired {
		resolution := projectSpecificationApplicabilityResolution{
			kind:            projectSpecificationScopeChoiceRequired,
			projectRoot:     basis.projectRoot,
			request:         request,
			basis:           basis,
			availableScopes: available,
		}
		if !resolution.Valid() {
			return projectSpecificationApplicabilityResolution{}, fmt.Errorf(
				"project specification scope-selection result is invalid",
			)
		}
		return resolution, nil
	}
	resolution := projectSpecificationApplicabilityResolution{
		kind:            projectSpecificationRequestedScopeNotFound,
		projectRoot:     basis.projectRoot,
		request:         request,
		basis:           basis,
		availableScopes: available,
	}
	if !resolution.Valid() {
		return projectSpecificationApplicabilityResolution{}, fmt.Errorf(
			"project specification missing-scope result is invalid",
		)
	}
	return resolution, nil
}

func canonicalScopeIDs(values []projectprofile.ScopeID) []projectprofile.ScopeID {
	copied := append([]projectprofile.ScopeID{}, values...)
	slices.SortFunc(copied, func(
		left projectprofile.ScopeID,
		right projectprofile.ScopeID,
	) int {
		if left.String() < right.String() {
			return -1
		}
		if left.String() > right.String() {
			return 1
		}
		return 0
	})
	return slices.Compact(copied)
}

func scopeIDValid(value projectprofile.ScopeID) bool {
	canonical, err := projectprofile.NewScopeID(value.String())
	return err == nil && canonical == value
}

func resolvedScopeMatchesRequest(
	selected projectprofile.ScopeID,
	available []projectprofile.ScopeID,
	request projectSpecificationScopeRequest,
) bool {
	if request.kind == projectSpecificationScopeExact {
		return selected == request.scopeID
	}
	return request.kind == projectSpecificationScopeAutomatic &&
		len(available) == 1 &&
		selected == available[0]
}

func validRuntimeApplicabilityMissingBasis(
	value profileadmissionsqlite.CapabilityApplicabilityMissingBasis,
) bool {
	return value == profileadmissionsqlite.MissingCurrentCanonicalProfileAdmission ||
		value == profileadmissionsqlite.MissingIntegrityValidProfileBasis
}
