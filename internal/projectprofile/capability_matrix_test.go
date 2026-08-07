package projectprofile_test

import (
	"slices"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func TestCapabilityApplicabilityMatrixIsPerScopeAndCanonical(t *testing.T) {
	payload := mustCapabilityMatrixPayload(
		t,
		[]projectprofile.RealizationScope{
			mustSoftwareScope(t, "software-z"),
			mustNonSoftwareScope(t, "documents-a"),
		},
	)
	matrix, err := projectprofile.ResolveCapabilityApplicabilityMatrix(payload)
	if err != nil {
		t.Fatalf("ResolveCapabilityApplicabilityMatrix: %v", err)
	}
	if !matrix.Valid() {
		t.Fatal("resolved capability matrix is invalid")
	}
	scopeIDs := matrix.ScopeIDs()
	if len(scopeIDs) != 2 ||
		scopeIDs[0].String() != "documents-a" ||
		scopeIDs[1].String() != "software-z" {
		t.Fatalf("scope IDs = %#v", scopeIDs)
	}
	capabilities := projectprofile.KnownCapabilities()
	entries := matrix.Entries()
	if len(entries) != len(scopeIDs)*len(capabilities) {
		t.Fatalf("entry count = %d, want %d", len(entries), len(scopeIDs)*len(capabilities))
	}
	assertCapabilityResult(
		t,
		matrix,
		"documents-a",
		projectprofile.SoftwareSystemSpecCapability,
		projectprofile.CapabilityNotApplicable,
	)
	assertCapabilityResult(
		t,
		matrix,
		"documents-a",
		projectprofile.FPFQueryCapability,
		projectprofile.CapabilityRequired,
	)
	assertCapabilityResult(
		t,
		matrix,
		"software-z",
		projectprofile.SoftwareSystemSpecCapability,
		projectprofile.CapabilityRequired,
	)
	assertCapabilityResult(
		t,
		matrix,
		"software-z",
		projectprofile.CodeDoctrineAndIndexCapability,
		projectprofile.CapabilityRequired,
	)
}

func TestNonSoftwareMatrixRetainsGeneralCapabilitiesWithoutSWECapabilities(
	t *testing.T,
) {
	payload := mustCapabilityMatrixPayload(
		t,
		[]projectprofile.RealizationScope{
			mustNonSoftwareScope(t, "knowledge-model"),
		},
	)
	matrix, err := projectprofile.ResolveCapabilityApplicabilityMatrix(payload)
	if err != nil {
		t.Fatalf("ResolveCapabilityApplicabilityMatrix: %v", err)
	}
	required := []projectprofile.Capability{
		projectprofile.FPFQueryCapability,
		projectprofile.ProjectStatusCapability,
		projectprofile.TargetSystemSpecCapability,
		projectprofile.TermMapCapability,
		projectprofile.TypedProjectMemoryCapability,
	}
	notApplicable := []projectprofile.Capability{
		projectprofile.CodeDoctrineAndIndexCapability,
		projectprofile.ProcessChecksCapability,
		projectprofile.SoftwareSystemSpecCapability,
		projectprofile.SWEMethodPackCapability,
	}
	assertCapabilitiesHaveKind(
		t,
		matrix,
		"knowledge-model",
		required,
		projectprofile.CapabilityRequired,
	)
	assertCapabilitiesHaveKind(
		t,
		matrix,
		"knowledge-model",
		notApplicable,
		projectprofile.CapabilityNotApplicable,
	)
}

func TestTargetSystemSpecApplicabilityDoesNotDependOnEntityRelation(
	t *testing.T,
) {
	entityRef, err := projectprofile.NewEntityRef("entity:target")
	if err != nil {
		t.Fatalf("NewEntityRef: %v", err)
	}
	software, err := projectprofile.NewSoftwareRealization(
		mustScopeID(t, "software-target"),
		projectprofile.NewReferencedEntity(entityRef),
	)
	if err != nil {
		t.Fatalf("NewSoftwareRealization: %v", err)
	}
	nonSoftware, err := projectprofile.NewNonSoftwareRealization(
		mustScopeID(t, "model-target"),
		projectprofile.NewReferencedEntity(entityRef),
		projectprofile.UnspecifiedKindOrientation{},
		[]projectprofile.SourceUnitRef{},
		[]projectprofile.SpecSectionRef{},
	)
	if err != nil {
		t.Fatalf("NewNonSoftwareRealization: %v", err)
	}
	payload := mustCapabilityMatrixPayload(
		t,
		[]projectprofile.RealizationScope{
			software,
			nonSoftware,
			mustSoftwareScope(t, "software-without-target"),
			mustNonSoftwareScope(t, "model-without-target"),
		},
	)
	matrix, err := projectprofile.ResolveCapabilityApplicabilityMatrix(payload)
	if err != nil {
		t.Fatalf("ResolveCapabilityApplicabilityMatrix: %v", err)
	}
	for _, scopeID := range []string{
		"model-target",
		"model-without-target",
		"software-target",
		"software-without-target",
	} {
		assertCapabilityResult(
			t,
			matrix,
			scopeID,
			projectprofile.TargetSystemSpecCapability,
			projectprofile.CapabilityRequired,
		)
		entry, found := matrix.Entry(
			mustScopeID(t, scopeID),
			projectprofile.TargetSystemSpecCapability,
		)
		if !found {
			t.Fatalf("target-system applicability for %q is absent", scopeID)
		}
		if missing, present := entry.MissingBasis(); present {
			t.Fatalf(
				"target-system applicability for %q retained missing basis %q",
				scopeID,
				missing,
			)
		}
	}
}

func TestCapabilityApplicabilityMatrixDoesNotExposeMutableEntries(t *testing.T) {
	payload := mustCapabilityMatrixPayload(
		t,
		[]projectprofile.RealizationScope{
			mustSoftwareScope(t, "software"),
		},
	)
	matrix, err := projectprofile.ResolveCapabilityApplicabilityMatrix(payload)
	if err != nil {
		t.Fatalf("ResolveCapabilityApplicabilityMatrix: %v", err)
	}
	entries := matrix.Entries()
	slices.Reverse(entries)
	if !matrix.Valid() {
		t.Fatal("mutating returned entries changed the matrix")
	}
	first := matrix.Entries()[0]
	if first.Capability() != projectprofile.CodeDoctrineAndIndexCapability {
		t.Fatalf("first capability = %q", first.Capability())
	}
}

func TestZeroCapabilityApplicabilityValuesAreInvalid(t *testing.T) {
	if (projectprofile.CapabilityApplicabilityMatrix{}).Valid() {
		t.Fatal("zero capability matrix is valid")
	}
	if (projectprofile.CapabilityApplicabilityEntry{}).Valid() {
		t.Fatal("zero capability entry is valid")
	}
}

func mustCapabilityMatrixPayload(
	t *testing.T,
	values []projectprofile.RealizationScope,
) projectprofile.ProfileDeclarationPayload {
	t.Helper()
	scopes := mustScopeSet(t, values)
	payload, err := projectprofile.NewProfileDeclarationPayload(scopes)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload: %v", err)
	}
	return payload
}

func assertCapabilitiesHaveKind(
	t *testing.T,
	matrix projectprofile.CapabilityApplicabilityMatrix,
	rawScopeID string,
	capabilities []projectprofile.Capability,
	want projectprofile.CapabilityApplicabilityKind,
) {
	t.Helper()
	for _, capability := range capabilities {
		assertCapabilityResult(t, matrix, rawScopeID, capability, want)
	}
}

func assertCapabilityResult(
	t *testing.T,
	matrix projectprofile.CapabilityApplicabilityMatrix,
	rawScopeID string,
	capability projectprofile.Capability,
	want projectprofile.CapabilityApplicabilityKind,
) {
	t.Helper()
	scopeID := mustScopeID(t, rawScopeID)
	entry, found := matrix.Entry(scopeID, capability)
	if !found {
		t.Fatalf("entry %s/%s is absent", rawScopeID, capability)
	}
	if !entry.Valid() {
		t.Fatalf("entry %s/%s is invalid", rawScopeID, capability)
	}
	if entry.Kind() != want {
		t.Fatalf("entry %s/%s = %q, want %q", rawScopeID, capability, entry.Kind(), want)
	}
}
