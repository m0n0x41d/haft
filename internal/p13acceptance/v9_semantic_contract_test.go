package p13acceptance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func TestCurrentV9SpecCarriersValidateWithoutFindings(t *testing.T) {
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}

	set, err := project.LoadProjectSpecificationSet(root)
	if err != nil {
		t.Fatalf("load current project specification carriers: %v", err)
	}
	report := specflow.ValidateCarrierSpecificationSet(set)
	if report.Summary.StructuralFindings != 0 || report.Summary.SemanticFindings != 0 {
		t.Fatalf(
			"current v9 specification carriers are not validation-clean: structural=%+v semantic=%+v",
			report.Structural.Findings,
			report.Semantic.Findings,
		)
	}
}

func TestCurrentV9CarriersKeepAliasOnlyIdentityAndExactWorkAttribution(t *testing.T) {
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}

	software := readV9SemanticCarrier(t, root, ".haft/specs/software-system.md")
	target := readV9SemanticCarrier(t, root, ".haft/specs/target-system.md")
	terms := readV9SemanticCarrier(t, root, ".haft/specs/term-map.md")
	readme := readV9SemanticCarrier(t, root, "README.md")
	profileWork := readV9SemanticCarrier(t, root, "internal/projectprofile/v1_work.go")
	profilePreparation := readV9SemanticCarrier(t, root, "internal/profiledeclarationpreparation/semantic.go")
	casWork := readV9SemanticCarrier(t, root, "internal/projecttypeenvselectioneffect/cas_work.go")

	for path, content := range map[string]string{
		".haft/specs/software-system.md": software,
		".haft/specs/target-system.md":   target,
		".haft/specs/term-map.md":        terms,
	} {
		if strings.Contains(content, "performedBy") {
			t.Fatalf("current v9 carrier %s attributes Work through legacy performedBy", path)
		}
	}

	for _, required := range []string{
		"actualPerformerSystem",
		"coveringRoleAssignment",
		"performedUnderAssignment",
	} {
		if !strings.Contains(software, required) || !strings.Contains(terms, required) {
			t.Fatalf("current Work-attribution carriers omit %q", required)
		}
	}
	for _, required := range []string{
		"ActualPerformerSystemRef",
		"CoveringRoleAssignmentRef",
		"PerformedUnderAssignment",
	} {
		if !strings.Contains(profileWork, required) {
			t.Fatalf("ProfileOnboarding Work domain omits current semantic API %q", required)
		}
	}
	for _, required := range []string{
		".PerformedUnderAssignment(",
		".ActualPerformer(",
	} {
		if !strings.Contains(profilePreparation, required) {
			t.Fatalf("new profile-onboarding Work construction omits %q", required)
		}
	}
	for _, legacyConstruction := range []string{".PerformedBy(", ".ExecutedWithin("} {
		if strings.Contains(profilePreparation, legacyConstruction) {
			t.Fatalf("new profile-onboarding Work construction uses legacy %q", legacyConstruction)
		}
	}
	for _, required := range []string{
		"ActualPerformerSystem",
		"CoveringRoleAssignment",
		"Keep the v1 canonical byte order stable",
	} {
		if !strings.Contains(casWork, required) {
			t.Fatalf("CAS Work domain omits current semantic compatibility boundary %q", required)
		}
	}

	for path, content := range map[string]string{
		".haft/specs/software-system.md": software,
		".haft/specs/term-map.md":        terms,
		"README.md":                      readme,
	} {
		for _, required := range []string{"admit_alias", "supersede_alias"} {
			if !strings.Contains(content, required) {
				t.Fatalf("current v9 identity carrier %s omits %q", path, required)
			}
		}
	}
	if strings.Contains(terms, "for an explicitly reviewed, append-only merge or") {
		t.Fatal("current IdentityChange term still promises public v9 merge/split")
	}
	readmeWords := strings.Join(strings.Fields(readme), " ")
	for _, required := range []string{
		"`workflow.state`",
		"`health`",
		"is not a release readiness claim",
	} {
		if !strings.Contains(readmeWords, required) {
			t.Fatalf("README omits spec workflow/health boundary %q", required)
		}
	}

	if !strings.Contains(software, "EpistemeConstitutionRelation signature must require exactly one ClaimGraphSlot") {
		t.Fatal("SoftwareSystemSpec does not scope ClaimGraphSlot to EpistemeConstitutionRelation")
	}
	if !strings.Contains(terms, "EpistemeConstitutionRelationSignature") {
		t.Fatal("term map does not scope ClaimGraphCodecV1 to EpistemeConstitutionRelationSignature")
	}
	for path, content := range map[string]string{
		".haft/specs/software-system.md": software,
		".haft/specs/term-map.md":        terms,
	} {
		if strings.Contains(content, "every supported C.2.1 episteme") {
			t.Fatalf("current v9 carrier %s retains family-wide ClaimGraph inference", path)
		}
	}
}

func readV9SemanticCarrier(t *testing.T, root string, relative string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatalf("read %s: %v", relative, err)
	}
	return string(content)
}
