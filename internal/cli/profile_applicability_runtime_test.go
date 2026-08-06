package cli

import (
	"context"
	"strings"
	"testing"

	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func TestCanonicalProjectSpecificationApplicabilityIsUnderdeterminedWithoutAdmission(
	t *testing.T,
) {
	fixture := newCLIProfileOnboardLedgerFixture(t)

	resolution, err := resolveCanonicalProjectSpecificationApplicability(
		context.Background(),
		fixture.root,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatalf("resolveCanonicalProjectSpecificationApplicability: %v", err)
	}
	if resolution.Kind() != projectSpecificationProfileUnderdetermined {
		t.Fatalf("resolution kind = %q", resolution.Kind())
	}
	if !resolution.Valid() {
		t.Fatal("profile-underdetermined resolution is invalid")
	}
	if resolution.ProjectRoot().String() != fixture.root {
		t.Fatalf(
			"resolution project root = %q, want %q",
			resolution.ProjectRoot().String(),
			fixture.root,
		)
	}
	missingBasis, ok := resolution.MissingBasis()
	if !ok ||
		missingBasis != profileadmissionsqlite.MissingCurrentCanonicalProfileAdmission {
		t.Fatalf("missing basis = %q, ok=%v", missingBasis, ok)
	}
}

func TestProjectSpecificationScopeSelectionDoesNotChooseMixedByOrder(
	t *testing.T,
) {
	t.Parallel()

	matrix := mustCLIProjectCapabilityMatrix(
		t,
		[]projectprofile.RealizationScope{
			mustCLIProjectSoftwareScope(t, "software"),
			mustCLIProjectNonSoftwareScope(t, "documents"),
		},
	)

	selection, err := selectProjectSpecificationScope(
		matrix,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatalf("selectProjectSpecificationScope: %v", err)
	}
	if selection.kind != projectSpecificationScopeSelectionRequired {
		t.Fatalf("selection kind = %q, want selection_required", selection.kind)
	}
	available := selection.AvailableScopeIDs()
	if len(available) != 2 ||
		available[0].String() != "documents" ||
		available[1].String() != "software" {
		t.Fatalf("available scopes = %#v", available)
	}
}

func TestProjectSpecificationScopeSelectionUsesOnlyCanonicalSingleton(
	t *testing.T,
) {
	t.Parallel()

	matrix := mustCLIProjectCapabilityMatrix(
		t,
		[]projectprofile.RealizationScope{
			mustCLIProjectSoftwareScope(t, "software"),
		},
	)

	selection, err := selectProjectSpecificationScope(
		matrix,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatalf("selectProjectSpecificationScope: %v", err)
	}
	selected, ok := selection.SelectedScopeID()
	if !ok || selected.String() != "software" {
		t.Fatalf("selected scope = %q, ok=%v", selected.String(), ok)
	}
}

func TestProjectSpecificationScopeSelectionAcceptsExactMixedScope(
	t *testing.T,
) {
	t.Parallel()

	matrix := mustCLIProjectCapabilityMatrix(
		t,
		[]projectprofile.RealizationScope{
			mustCLIProjectSoftwareScope(t, "software"),
			mustCLIProjectNonSoftwareScope(t, "documents"),
		},
	)
	documentsID, err := projectprofile.NewScopeID("documents")
	if err != nil {
		t.Fatal(err)
	}
	request, err := exactProjectSpecificationScopeRequest(documentsID)
	if err != nil {
		t.Fatal(err)
	}

	selection, err := selectProjectSpecificationScope(matrix, request)
	if err != nil {
		t.Fatalf("selectProjectSpecificationScope: %v", err)
	}
	selected, ok := selection.SelectedScopeID()
	if !ok || selected != documentsID {
		t.Fatalf("selected scope = %q, ok=%v", selected.String(), ok)
	}
}

func TestProjectSpecificationScopeSelectionReportsUnknownExactScope(
	t *testing.T,
) {
	t.Parallel()

	matrix := mustCLIProjectCapabilityMatrix(
		t,
		[]projectprofile.RealizationScope{
			mustCLIProjectSoftwareScope(t, "software"),
		},
	)
	unknownID, err := projectprofile.NewScopeID("unknown")
	if err != nil {
		t.Fatal(err)
	}
	request, err := exactProjectSpecificationScopeRequest(unknownID)
	if err != nil {
		t.Fatal(err)
	}

	selection, err := selectProjectSpecificationScope(matrix, request)
	if err != nil {
		t.Fatalf("selectProjectSpecificationScope: %v", err)
	}
	if selection.kind != projectSpecificationScopeNotFound {
		t.Fatalf("selection kind = %q, want not_found", selection.kind)
	}
}

func TestProjectSpecificationApplicabilityRequiresScopeForMixedMatrix(
	t *testing.T,
) {
	t.Parallel()

	matrix := mustCLIProjectCapabilityMatrix(
		t,
		[]projectprofile.RealizationScope{
			mustCLIProjectSoftwareScope(t, "software"),
			mustCLIProjectNonSoftwareScope(t, "documents"),
		},
	)
	basis := mustCLICanonicalProfileApplicabilityBasis(t, matrix)

	resolution, err := projectSpecificationApplicabilityFromMatrix(
		matrix,
		basis,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatalf("projectSpecificationApplicabilityFromMatrix: %v", err)
	}
	if !resolution.Valid() ||
		resolution.Kind() != projectSpecificationScopeChoiceRequired {
		t.Fatalf("mixed resolution = %#v", resolution)
	}
	if len(resolution.AvailableScopeIDs()) != 2 {
		t.Fatalf("available scopes = %#v", resolution.AvailableScopeIDs())
	}
}

func TestProjectSpecificationApplicabilityResolvesExactMixedScope(
	t *testing.T,
) {
	t.Parallel()

	matrix := mustCLIProjectCapabilityMatrix(
		t,
		[]projectprofile.RealizationScope{
			mustCLIProjectSoftwareScope(t, "software"),
			mustCLIProjectNonSoftwareScope(t, "documents"),
		},
	)
	basis := mustCLICanonicalProfileApplicabilityBasis(t, matrix)
	documentsID, err := projectprofile.NewScopeID("documents")
	if err != nil {
		t.Fatal(err)
	}
	request, err := exactProjectSpecificationScopeRequest(documentsID)
	if err != nil {
		t.Fatal(err)
	}

	resolution, err := projectSpecificationApplicabilityFromMatrix(
		matrix,
		basis,
		request,
	)
	if err != nil {
		t.Fatalf("projectSpecificationApplicabilityFromMatrix: %v", err)
	}
	applicability, resolvedBasis, ok := resolution.Resolved()
	if !ok || applicability.ScopeID() != documentsID {
		t.Fatalf("resolved applicability = %#v, ok=%v", applicability, ok)
	}
	if resolvedBasis != basis {
		t.Fatal("resolved applicability lost canonical admission provenance")
	}
}

func TestResolvedProjectSpecificationApplicabilityRejectsMismatchedProfileDigest(
	t *testing.T,
) {
	t.Parallel()

	matrix := mustCLIProjectCapabilityMatrix(
		t,
		[]projectprofile.RealizationScope{
			mustCLIProjectSoftwareScope(t, "software"),
		},
	)
	basis := mustCLICanonicalProfileApplicabilityBasis(t, matrix)
	request := automaticProjectSpecificationScopeRequest()
	resolution, err := projectSpecificationApplicabilityFromMatrix(
		matrix,
		basis,
		request,
	)
	if err != nil {
		t.Fatalf("projectSpecificationApplicabilityFromMatrix: %v", err)
	}
	otherDigest, err := projectprofile.NewContentDigest(
		"sha256:" + strings.Repeat("b", 64),
	)
	if err != nil {
		t.Fatal(err)
	}
	resolution.basis.payloadDigest = otherDigest

	if resolution.Valid() {
		t.Fatal("resolved applicability accepted provenance from another profile")
	}
	if _, _, resolved := resolution.Resolved(); resolved {
		t.Fatal("mismatched profile provenance remained observable as resolved")
	}
}

func TestResolvedProjectSpecificationApplicabilityBindsOriginatingScopeRequest(
	t *testing.T,
) {
	t.Parallel()

	matrix := mustCLIProjectCapabilityMatrix(
		t,
		[]projectprofile.RealizationScope{
			mustCLIProjectSoftwareScope(t, "software"),
			mustCLIProjectNonSoftwareScope(t, "documents"),
		},
	)
	basis := mustCLICanonicalProfileApplicabilityBasis(t, matrix)
	request := mustExactProjectSpecificationScopeRequest(t, "documents")
	resolution, err := projectSpecificationApplicabilityFromMatrix(
		matrix,
		basis,
		request,
	)
	if err != nil {
		t.Fatalf("projectSpecificationApplicabilityFromMatrix: %v", err)
	}
	resolution.request = automaticProjectSpecificationScopeRequest()

	if resolution.Valid() {
		t.Fatal("resolved applicability accepted a different originating request")
	}
}

func TestSQLFirstCanonicalProfileReadStopsBeforeCarriersWhenProfileIsAbsent(
	t *testing.T,
) {
	fixture := newCLIProfileOnboardLedgerFixture(t)

	specSet, resolution, err := loadProjectSpecificationSetSQLFirstFromCanonicalProfile(
		context.Background(),
		fixture.root,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatalf("loadProjectSpecificationSetSQLFirstFromCanonicalProfile: %v", err)
	}
	if resolution.Kind() != projectSpecificationProfileUnderdetermined {
		t.Fatalf("resolution kind = %q", resolution.Kind())
	}
	if len(specSet.Documents) != 0 ||
		len(specSet.Sections) != 0 ||
		len(specSet.Findings) != 0 {
		t.Fatalf("unresolved profile read touched specification carriers: %#v", specSet)
	}
}

func mustCLICanonicalProfileApplicabilityBasis(
	t *testing.T,
	matrix projectprofile.CapabilityApplicabilityMatrix,
) canonicalProfileApplicabilityBasis {
	t.Helper()
	projectRoot, err := projectprofile.NewProjectRootV1("/tmp/haft-profile")
	if err != nil {
		t.Fatal(err)
	}
	admissionRef, err := projectprofile.NewProfileDeclarationAdmissionRecordRef(
		"profile-admission:test",
	)
	if err != nil {
		t.Fatal(err)
	}
	admissionDigest, err := projectprofile.NewContentDigest(
		"sha256:" + strings.Repeat("a", 64),
	)
	if err != nil {
		t.Fatal(err)
	}
	return canonicalProfileApplicabilityBasis{
		projectRoot:           projectRoot,
		origin:                projectprofile.ProfileAdmissionOriginExplicitOperator,
		admissionRecordRef:    admissionRef,
		admissionRecordDigest: admissionDigest,
		payloadDigest:         matrix.ProfilePayloadDigest(),
		ledgerRevision:        projectprofile.NewLedgerRevision(1),
	}
}

func mustCLIProjectCapabilityMatrix(
	t *testing.T,
	scopes []projectprofile.RealizationScope,
) projectprofile.CapabilityApplicabilityMatrix {
	t.Helper()
	scopeSet, err := projectprofile.NewScopeSet(scopes)
	if err != nil {
		t.Fatalf("NewScopeSet: %v", err)
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(scopeSet)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload: %v", err)
	}
	matrix, err := projectprofile.ResolveCapabilityApplicabilityMatrix(payload)
	if err != nil {
		t.Fatalf("ResolveCapabilityApplicabilityMatrix: %v", err)
	}
	return matrix
}

func mustCLIProjectSoftwareScope(
	t *testing.T,
	rawScopeID string,
) projectprofile.SoftwareRealization {
	t.Helper()
	scopeID, err := projectprofile.NewScopeID(rawScopeID)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := projectprofile.NewSoftwareRealization(
		scopeID,
		projectprofile.NoEntityReference{},
	)
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

func mustCLIProjectNonSoftwareScope(
	t *testing.T,
	rawScopeID string,
) projectprofile.NonSoftwareRealization {
	t.Helper()
	scopeID, err := projectprofile.NewScopeID(rawScopeID)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := projectprofile.NewNonSoftwareRealization(
		scopeID,
		projectprofile.NoEntityReference{},
		projectprofile.UnspecifiedKindOrientation{},
		nil,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	return scope
}
