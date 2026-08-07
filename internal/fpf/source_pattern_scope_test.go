package fpf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSourceUnits_PatternScopeDeclarationsAreExactAddressableUnits(t *testing.T) {
	specPath := filepath.Join("..", "..", "data", "FPF", "FPF-Spec.md")
	readmePath := filepath.Join("..", "..", "data", "FPF", "Readme.md")
	markdown, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read FPF specification: %v", err)
	}

	declarationCount := countPatternScopeDeclarations(markdown)
	units, err := LoadSourceUnits(readmePath, specPath, "pattern-scope-test")
	if err != nil {
		t.Fatalf("LoadSourceUnits() error: %v", err)
	}

	scopes := sourceUnitsByRole(units, SourceUnitRolePatternScope)
	if len(scopes) != declarationCount {
		t.Fatalf("pattern scope units = %d, want all %d source declarations", len(scopes), declarationCount)
	}
	if len(scopes) == 0 {
		t.Fatal("production FPF source exposes no PatternScopeId declarations")
	}

	scope := findSourceUnitBySourceID(t, scopes, "G.4:Ext.EvidenceGraphWiring")
	if scope.SourceID != "G.4:Ext.EvidenceGraphWiring" {
		t.Fatalf("pattern scope source id = %q, want canonical source spelling", scope.SourceID)
	}
	if scope.PatternID != "G.4" || scope.ParentPatternID != "G.4" {
		t.Fatalf("pattern scope parent = (%q, %q), want G.4", scope.PatternID, scope.ParentPatternID)
	}
	for _, field := range []string{"PatternScopeId", "GPatternExtensionId", "GoverningPatternId"} {
		if !strings.Contains(scope.Body, field) {
			t.Fatalf("pattern scope body omits source-owned field %s", field)
		}
	}
	if !containsSourceString(scope.DirectRefs, "G.6") {
		t.Fatalf("pattern scope direct refs = %#v, want governing G.6 source", scope.DirectRefs)
	}
	if containsSourceString(scope.DirectRefs, "G.4:Ext.EvidenceGraphWiring") {
		t.Fatalf("pattern scope direct refs retain self reference: %#v", scope.DirectRefs)
	}
	if scope.Provenance.StartLine <= 0 || scope.Provenance.EndLine < scope.Provenance.StartLine {
		t.Fatalf("pattern scope provenance = %#v", scope.Provenance)
	}
}

func TestValidateSourceReferences_PatternScopeResolvesTOCDirectReference(t *testing.T) {
	units := []SourceUnit{
		{Role: SourceUnitRolePatternBody, PatternID: "G.4"},
		{Role: SourceUnitRolePatternBody, PatternID: "G.6"},
		{Role: SourceUnitRoleTOCRow, PatternID: "G.4", DirectRefs: []string{"G.4:Ext.EvidenceGraphWiring"}},
		{Role: SourceUnitRoleTOCRow, PatternID: "G.6"},
		{
			Role:            SourceUnitRolePatternScope,
			SourceID:        "G.4:Ext.EvidenceGraphWiring",
			PatternID:       "G.4",
			ParentPatternID: "G.4",
		},
	}

	err := validateSourceReferences(units, map[string]SpecCatalogEntry{})
	if err != nil {
		t.Fatalf("declared PatternScopeId did not resolve ToC direct reference: %v", err)
	}

	withoutScope := append([]SourceUnit(nil), units[:4]...)
	err = validateSourceReferences(withoutScope, map[string]SpecCatalogEntry{})
	if err == nil || !strings.Contains(err.Error(), "source_reference_unresolved") {
		t.Fatalf("missing PatternScopeId error = %v, want fail-closed unresolved reference", err)
	}
}

func countPatternScopeDeclarations(markdown []byte) int {
	count := 0
	lines := splitPatternAtlasLines(markdown)
	for _, line := range lines {
		_, found := extractPatternScopeIDField(line)
		if found {
			count++
		}
	}
	return count
}

func sourceUnitsByRole(units []SourceUnit, role SourceUnitRole) []SourceUnit {
	filtered := make([]SourceUnit, 0)
	for _, unit := range units {
		if unit.Role == role {
			filtered = append(filtered, unit)
		}
	}
	return filtered
}

func findSourceUnitBySourceID(t *testing.T, units []SourceUnit, sourceID string) SourceUnit {
	t.Helper()
	for _, unit := range units {
		if unit.SourceID == sourceID {
			return unit
		}
	}
	t.Fatalf("source unit %s not found", sourceID)
	return SourceUnit{}
}
