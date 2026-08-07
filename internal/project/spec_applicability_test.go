package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func TestNonSoftwareSpecificationProjectionKeepsTargetAndOmitsSoftwareCarrier(
	t *testing.T,
) {
	applicability := mustSpecificationApplicability(
		t,
		mustProjectProfileNonSoftwareScope(t, "documents"),
		"documents",
	)
	report, err := CheckSpecDocumentsForScope(
		specificationApplicabilityDocuments(),
		applicability,
	)
	if err != nil {
		t.Fatalf("CheckSpecDocumentsForScope: %v", err)
	}
	if hasSpecCheckFinding(report, "profile_capability_applicability_underdetermined") {
		t.Fatalf("non-software report retained a target-relation gate: %+v", report.Findings)
	}
	if len(report.Documents) != 2 {
		t.Fatalf("non-software documents = %#v, want TargetSystemSpec and TermMap", report.Documents)
	}
	if report.Documents[0].Kind != string(SpecDocumentKindTargetSystem) ||
		report.Documents[1].Kind != string(SpecDocumentKindTermMap) {
		t.Fatalf("non-software document kinds = %#v", report.Documents)
	}
	excluded := applicability.ExcludedDocumentKinds()
	if len(excluded) != 1 ||
		excluded[0] != SpecDocumentKindSoftwareSystem {
		t.Fatalf("excluded document kinds = %#v", excluded)
	}
	if len(applicability.UnderdeterminedDocumentKinds()) != 0 {
		t.Fatalf("underdetermined document kinds = %#v", applicability.UnderdeterminedDocumentKinds())
	}
}

func TestSoftwareSpecificationProjectionKeepsSoftwareReadinessFinding(
	t *testing.T,
) {
	applicability := mustSpecificationApplicability(
		t,
		mustProjectProfileSoftwareScope(t, "software"),
		"software",
	)
	report, err := CheckSpecDocumentsForScope(
		specificationApplicabilityDocuments(),
		applicability,
	)
	if err != nil {
		t.Fatalf("CheckSpecDocumentsForScope: %v", err)
	}
	if !hasSpecCheckFinding(report, "spec_carrier_no_active_sections") {
		t.Fatalf("software report findings = %+v", report.Findings)
	}
	if len(report.Documents) != 3 {
		t.Fatalf("software documents = %#v, want TargetSystemSpec, SoftwareSystemSpec, and TermMap", report.Documents)
	}
	if len(applicability.ExcludedDocumentKinds()) != 0 {
		t.Fatalf("software excluded kinds = %#v", applicability.ExcludedDocumentKinds())
	}
}

func TestScopedSpecificationProjectionDropsLegacyEnablingDocument(t *testing.T) {
	applicability := mustSpecificationApplicability(
		t,
		mustProjectProfileSoftwareScope(t, "software"),
		"software",
	)
	documents := append(
		specificationApplicabilityDocuments(),
		SpecDocumentInput{
			Path: ".haft/specs/enabling-system.md",
			Kind: string(SpecDocumentKindEnablingSystem),
			Content: validSpecSectionCarrier(
				"ES.legacy.001",
				"enabling.role",
				"active",
			),
		},
	)

	specSet, err := ProjectSpecificationSetFromDocumentsForScope(
		documents,
		applicability,
	)
	if err != nil {
		t.Fatalf("ProjectSpecificationSetFromDocumentsForScope: %v", err)
	}
	for _, document := range specSet.Documents {
		if document.Kind == SpecDocumentKindEnablingSystem {
			t.Fatalf("scoped projection retained legacy document: %#v", specSet.Documents)
		}
	}
}

func TestMixedProfileDerivesDifferentSpecificationSetsPerScope(t *testing.T) {
	scopes := []projectprofile.RealizationScope{
		mustProjectProfileSoftwareScope(t, "software"),
		mustProjectProfileNonSoftwareScope(t, "documents"),
	}
	matrix := mustProjectProfileCapabilityMatrix(t, scopes)
	software := mustSpecificationApplicabilityFromMatrix(t, matrix, "software")
	documents := mustSpecificationApplicabilityFromMatrix(t, matrix, "documents")
	softwareSet, err := ProjectSpecificationSetFromDocumentsForScope(
		specificationApplicabilityDocuments(),
		software,
	)
	if err != nil {
		t.Fatalf("software projection: %v", err)
	}
	documentSet, err := ProjectSpecificationSetFromDocumentsForScope(
		specificationApplicabilityDocuments(),
		documents,
	)
	if err != nil {
		t.Fatalf("documents projection: %v", err)
	}
	if len(softwareSet.Documents) != 3 {
		t.Fatalf("software document count = %d, want 3", len(softwareSet.Documents))
	}
	if len(documentSet.Documents) != 2 {
		t.Fatalf("document/model document count = %d, want 2", len(documentSet.Documents))
	}
	if software.ProfilePayloadDigest() != documents.ProfilePayloadDigest() {
		t.Fatal("mixed scope projections lost their common canonical profile basis")
	}
}

func TestZeroSpecificationApplicabilityCannotSuppressFindings(t *testing.T) {
	_, err := ProjectSpecificationSetFromDocumentsForScope(
		specificationApplicabilityDocuments(),
		ProjectSpecificationSetApplicability{},
	)
	if err == nil {
		t.Fatal("zero applicability suppressed specification members")
	}
}

func TestNonSoftwareCarrierLoaderDoesNotReadLegacySoftwareCarrier(
	t *testing.T,
) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".haft", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(
		t,
		filepath.Join(specDir, "enabling-system.md"),
		"# Historical software-only migration input\n",
	)
	writeFixture(
		t,
		filepath.Join(specDir, "target-system.md"),
		readinessSpecSection("TS.environment.001", "target.environment"),
	)
	writeFixture(
		t,
		filepath.Join(specDir, "term-map.md"),
		validTermMapCarrier(),
	)
	applicability := mustSpecificationApplicability(
		t,
		mustProjectProfileNonSoftwareScope(t, "documents"),
		"documents",
	)

	report, err := CheckSpecificationSetForScope(root, applicability)
	if err != nil {
		t.Fatalf("CheckSpecificationSetForScope: %v", err)
	}
	if hasSpecCheckFinding(report, SpecMigrationRequiredFindingCode) {
		t.Fatalf("non-software report retained legacy migration pressure: %+v", report.Findings)
	}
	if hasSpecCheckFinding(report, "spec_carrier_missing_file") ||
		hasSpecCheckFinding(report, "profile_capability_applicability_underdetermined") {
		t.Fatalf("non-software report retained a relation gate: %+v", report.Findings)
	}
	if len(report.Documents) != 2 ||
		report.Documents[0].Kind != string(SpecDocumentKindTargetSystem) ||
		report.Documents[1].Kind != string(SpecDocumentKindTermMap) {
		t.Fatalf("non-software documents = %#v, want TargetSystemSpec and TermMap", report.Documents)
	}
}

func TestSoftwareCarrierLoaderPreservesLegacyMigrationFinding(
	t *testing.T,
) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".haft", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(
		t,
		filepath.Join(specDir, "enabling-system.md"),
		readinessSpecSection("ES.role.001", "enabling.role"),
	)
	writeFixture(
		t,
		filepath.Join(specDir, "target-system.md"),
		readinessSpecSection("TS.environment.001", "target.environment"),
	)
	writeFixture(
		t,
		filepath.Join(specDir, "term-map.md"),
		validTermMapCarrier(),
	)
	applicability := mustSpecificationApplicability(
		t,
		mustProjectProfileSoftwareScope(t, "software"),
		"software",
	)

	report, err := CheckSpecificationSetForScope(root, applicability)
	if err != nil {
		t.Fatalf("CheckSpecificationSetForScope: %v", err)
	}
	if !hasSpecCheckFinding(report, SpecMigrationRequiredFindingCode) {
		t.Fatalf("software report omitted legacy migration pressure: %+v", report.Findings)
	}
	if hasSpecCheckFinding(report, "spec_carrier_missing_file") {
		t.Fatalf("legacy software fallback was not loaded: %+v", report.Findings)
	}
}

func specificationApplicabilityDocuments() []SpecDocumentInput {
	return []SpecDocumentInput{
		{
			Path: ".haft/specs/target-system.md",
			Kind: string(SpecDocumentKindTargetSystem),
			Content: validSpecSectionCarrier(
				"TS.environment.001",
				"environment-change",
				"active",
			),
		},
		{
			Path: ".haft/specs/software-system.md",
			Kind: string(SpecDocumentKindSoftwareSystem),
			Content: validSpecSectionCarrier(
				"SS.placeholder.001",
				"software.role",
				"draft",
			),
		},
		{
			Path:    ".haft/specs/term-map.md",
			Kind:    string(SpecDocumentKindTermMap),
			Content: validTermMapCarrier(),
		},
	}
}

func mustSpecificationApplicability(
	t *testing.T,
	scope projectprofile.RealizationScope,
	rawScopeID string,
) ProjectSpecificationSetApplicability {
	t.Helper()
	matrix := mustProjectProfileCapabilityMatrix(
		t,
		[]projectprofile.RealizationScope{scope},
	)
	return mustSpecificationApplicabilityFromMatrix(t, matrix, rawScopeID)
}

func mustSpecificationApplicabilityFromMatrix(
	t *testing.T,
	matrix projectprofile.CapabilityApplicabilityMatrix,
	rawScopeID string,
) ProjectSpecificationSetApplicability {
	t.Helper()
	scopeID, err := projectprofile.NewScopeID(rawScopeID)
	if err != nil {
		t.Fatalf("NewScopeID: %v", err)
	}
	applicability, err := DeriveProjectSpecificationSetApplicability(
		matrix,
		scopeID,
	)
	if err != nil {
		t.Fatalf("DeriveProjectSpecificationSetApplicability: %v", err)
	}
	return applicability
}

func mustProjectProfileCapabilityMatrix(
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

func mustProjectProfileSoftwareScope(
	t *testing.T,
	rawScopeID string,
) projectprofile.SoftwareRealization {
	t.Helper()
	scopeID, err := projectprofile.NewScopeID(rawScopeID)
	if err != nil {
		t.Fatalf("NewScopeID: %v", err)
	}
	scope, err := projectprofile.NewSoftwareRealization(
		scopeID,
		projectprofile.NoEntityReference{},
	)
	if err != nil {
		t.Fatalf("NewSoftwareRealization: %v", err)
	}
	return scope
}

func mustProjectProfileNonSoftwareScope(
	t *testing.T,
	rawScopeID string,
) projectprofile.NonSoftwareRealization {
	t.Helper()
	scopeID, err := projectprofile.NewScopeID(rawScopeID)
	if err != nil {
		t.Fatalf("NewScopeID: %v", err)
	}
	scope, err := projectprofile.NewNonSoftwareRealization(
		scopeID,
		projectprofile.NoEntityReference{},
		projectprofile.UnspecifiedKindOrientation{},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("NewNonSoftwareRealization: %v", err)
	}
	return scope
}
