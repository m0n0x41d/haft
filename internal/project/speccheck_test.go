package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckSpecDocumentsAcceptsValidSpecSet(t *testing.T) {
	report := CheckSpecDocuments([]SpecDocumentInput{
		{
			Path:    ".haft/specs/target-system.md",
			Kind:    "target-system",
			Content: validSpecSectionCarrier("TS.use.001", "environment-change", "active"),
		},
		{
			Path:    ".haft/specs/enabling-system.md",
			Kind:    "enabling-system",
			Content: validSpecSectionCarrier("ES.creator.001", "creator-role", "active"),
		},
		{
			Path:    ".haft/specs/term-map.md",
			Kind:    "term-map",
			Content: validTermMapCarrier(),
		},
	})

	if report.HasFindings() {
		t.Fatalf("report has findings: %+v", report.Findings)
	}
	if report.Summary.SpecSections != 2 {
		t.Fatalf("spec_sections = %d, want 2", report.Summary.SpecSections)
	}
	if report.Summary.ActiveSpecSections != 2 {
		t.Fatalf("active_spec_sections = %d, want 2", report.Summary.ActiveSpecSections)
	}
	if report.Summary.TermMapEntries != 1 {
		t.Fatalf("term_map_entries = %d, want 1", report.Summary.TermMapEntries)
	}
}

func TestCheckSpecDocumentsFindsMissingRequiredSpecSectionField(t *testing.T) {
	report := CheckSpecDocuments([]SpecDocumentInput{
		{
			Path: ".haft/specs/target-system.md",
			Kind: "target-system",
			Content: "## TS.use.001\n\n```yaml spec-section\n" +
				"id: TS.use.001\n" +
				"kind: environment-change\n" +
				"statement_type: definition\n" +
				"owner: human\n" +
				"status: active\n" +
				"```\n",
		},
	})

	if !hasSpecCheckFinding(report, "spec_section_missing_field") {
		t.Fatalf("report findings = %+v, want missing field finding", report.Findings)
	}
}

func TestCheckSpecDocumentsFindsTermMapEntryMissingTerm(t *testing.T) {
	report := CheckSpecDocuments([]SpecDocumentInput{
		{
			Path: ".haft/specs/term-map.md",
			Kind: "term-map",
			Content: "```yaml term-map\n" +
				"entries:\n" +
				"  - domain: enabling\n" +
				"    definition: A project with parseable specs.\n" +
				"```\n",
		},
	})

	if !hasSpecCheckFinding(report, "term_map_missing_term") {
		t.Fatalf("report findings = %+v, want missing term finding", report.Findings)
	}
}

func TestCheckSpecDocumentsAcceptsPlainYamlTermMapEntry(t *testing.T) {
	report := CheckSpecDocuments([]SpecDocumentInput{
		{
			Path: ".haft/specs/term-map.md",
			Kind: "term-map",
			Content: "```yaml\n" +
				"term: HarnessableProject\n" +
				"domain: enabling\n" +
				"definition: A project with active specs.\n" +
				"```\n",
		},
	})

	if report.HasFindings() {
		t.Fatalf("report has findings: %+v", report.Findings)
	}
	if report.Summary.TermMapEntries != 1 {
		t.Fatalf("term_map_entries = %d, want 1", report.Summary.TermMapEntries)
	}
}

func TestCheckSpecDocumentsAcceptsValidL15Shapes(t *testing.T) {
	report := CheckSpecDocuments([]SpecDocumentInput{
		{
			Path: ".haft/specs/target-system.md",
			Kind: "target-system",
			Content: "## TS.use.001\n\n" +
				"```yaml spec-section\n" +
				"id: TS.use.001\n" +
				"kind: acceptance\n" +
				"statement_type: evidence\n" +
				"claim_layer: object\n" +
				"owner: human\n" +
				"status: active\n" +
				"valid_until: 2026-07-24\n" +
				"depends_on:\n" +
				"  - TS.role.001\n" +
				"target_refs:\n" +
				"  - ES.test.001\n" +
				"evidence_required:\n" +
				"  - kind: review\n" +
				"    description: Human confirms acceptance still matches target-system intent.\n" +
				"```\n",
		},
		{
			Path: ".haft/specs/term-map.md",
			Kind: "term-map",
			Content: "```yaml term-map\n" +
				"entries:\n" +
				"  - term: HarnessableProject\n" +
				"    domain: enabling\n" +
				"    definition: A project with active specs.\n" +
				"    not:\n" +
				"      - repo with only README\n" +
				"    aliases:\n" +
				"      - harness-ready project\n" +
				"```\n",
		},
	})

	if report.HasFindings() {
		t.Fatalf("report has findings: %+v", report.Findings)
	}
	if report.Level != "L0/L1/L1.5" {
		t.Fatalf("level = %q, want L0/L1/L1.5", report.Level)
	}
}

func TestCheckSpecDocumentsValidatesTermMapShapeAndDuplicateAliases(t *testing.T) {
	report := CheckSpecDocuments([]SpecDocumentInput{
		{
			Path: ".haft/specs/term-map.md",
			Kind: "term-map",
			Content: "```yaml term-map\n" +
				"entries:\n" +
				"  - term: HarnessableProject\n" +
				"    domain: enabling\n" +
				"    definition: A project with active specs.\n" +
				"    not: tracker ticket\n" +
				"    aliases:\n" +
				"      - spec set\n" +
				"      - spec set\n" +
				"  - term: ProjectSpecificationSet\n" +
				"    domain: enabling\n" +
				"    definition: Project-local specs and workflow policy.\n" +
				"    aliases:\n" +
				"      - spec set\n" +
				"  - term: MissingPieces\n" +
				"```\n",
		},
	})

	assertSpecCheckFindingAt(t, report, "term_map_invalid_not", "$.entries[0].not")
	assertSpecCheckFindingAt(t, report, "term_map_duplicate_alias", "$.entries[0].aliases[1]")
	assertSpecCheckFindingAt(t, report, "term_map_duplicate_alias", "$.entries[1].aliases[0]")
	assertSpecCheckFindingAt(t, report, "term_map_missing_domain", "$.entries[2].domain")
	assertSpecCheckFindingAt(t, report, "term_map_missing_definition", "$.entries[2].definition")
}

func TestCheckSpecDocumentsValidatesSectionOptionalFieldShapes(t *testing.T) {
	report := CheckSpecDocuments([]SpecDocumentInput{
		{
			Path: ".haft/specs/target-system.md",
			Kind: "target-system",
			Content: "## TS.use.001\n\n" +
				"```yaml spec-section\n" +
				"id: TS.use.001\n" +
				"kind: environment-change\n" +
				"statement_type: definition\n" +
				"claim_layer: object\n" +
				"owner: human\n" +
				"status: active\n" +
				"valid_until: 07/24/2026\n" +
				"depends_on: TS.role.001\n" +
				"target_refs:\n" +
				"  - TS.role.001\n" +
				"  - \"\"\n" +
				"evidence_required:\n" +
				"  - kind: review\n" +
				"```\n",
		},
	})

	assertSpecCheckFindingAt(t, report, "spec_section_invalid_valid_until", "$.valid_until")
	assertSpecCheckFindingAt(t, report, "spec_section_invalid_depends_on", "$.depends_on")
	assertSpecCheckFindingAt(t, report, "spec_section_invalid_target_refs", "$.target_refs[1]")
	assertSpecCheckFindingAt(t, report, "spec_section_invalid_evidence_required", "$.evidence_required[0].description")
}

func TestCheckSpecDocumentsGuardsActiveTargetCarrierClaims(t *testing.T) {
	report := CheckSpecDocuments([]SpecDocumentInput{
		{
			Path: ".haft/specs/target-system.md",
			Kind: "target-system",
			Content: "## TS.use.001\n\n" +
				"```yaml spec-section\n" +
				"id: TS.use.001\n" +
				"kind: environment-change\n" +
				"statement_type: definition\n" +
				"claim_layer: carrier\n" +
				"owner: human\n" +
				"status: active\n" +
				"```\n",
		},
	})

	assertSpecCheckFindingAt(t, report, "spec_section_mixed_authority", "$.claim_layer")
}

func TestCheckSpecDocumentsAllowsExplicitCarrierClaimAllowance(t *testing.T) {
	report := CheckSpecDocuments([]SpecDocumentInput{
		{
			Path: ".haft/specs/target-system.md",
			Kind: "target-system",
			Content: "## TS.carrier.001\n\n" +
				"```yaml spec-section\n" +
				"id: TS.carrier.001\n" +
				"kind: boundaries\n" +
				"statement_type: explanation\n" +
				"claim_layer: carrier\n" +
				"owner: human\n" +
				"status: active\n" +
				"carrier_claim_allowed: true\n" +
				"```\n",
		},
	})

	if report.HasFindings() {
		t.Fatalf("report has findings: %+v", report.Findings)
	}
}

func TestCheckSpecificationSetFindsMissingTermMapFile(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".haft", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeSpecCheckFixture(t, filepath.Join(specDir, "target-system.md"), validSpecSectionCarrier("TS.use.001", "environment-change", "active"))
	writeSpecCheckFixture(t, filepath.Join(specDir, "enabling-system.md"), validSpecSectionCarrier("ES.creator.001", "creator-role", "active"))

	report, err := CheckSpecificationSet(root)
	if err != nil {
		t.Fatalf("CheckSpecificationSet: %v", err)
	}
	if !hasSpecCheckFinding(report, "term_map_missing_file") {
		t.Fatalf("report findings = %+v, want missing term-map file finding", report.Findings)
	}
}

func hasSpecCheckFinding(report SpecCheckReport, code string) bool {
	for _, finding := range report.Findings {
		if finding.Code != code {
			continue
		}

		return true
	}

	return false
}

func assertSpecCheckFindingAt(t *testing.T, report SpecCheckReport, code string, fieldPath string) {
	t.Helper()

	for _, finding := range report.Findings {
		if finding.Code != code {
			continue
		}
		if finding.FieldPath != fieldPath {
			continue
		}

		return
	}

	t.Fatalf("report findings = %+v, want code %q at %q", report.Findings, code, fieldPath)
}

func validSpecSectionCarrier(id string, kind string, status string) string {
	return "## " + id + "\n\n" +
		"```yaml spec-section\n" +
		"id: " + id + "\n" +
		"kind: " + kind + "\n" +
		"statement_type: definition\n" +
		"claim_layer: object\n" +
		"owner: human\n" +
		"status: " + status + "\n" +
		"```\n"
}

func validTermMapCarrier() string {
	return "```yaml term-map\n" +
		"entries:\n" +
		"  - term: HarnessableProject\n" +
		"    domain: enabling\n" +
		"    definition: A project with active specs.\n" +
		"```\n"
}

func writeSpecCheckFixture(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
