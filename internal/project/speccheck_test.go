package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestProjectSpecificationSetParsesCanonicalSectionFields(t *testing.T) {
	specSet := ProjectSpecificationSetFromDocuments([]SpecDocumentInput{
		{
			Path: ".haft/specs/target-system.md",
			Kind: "target-system",
			Content: "## TS.acceptance.001 Canonical fields\n\n" +
				"```yaml spec-section\n" +
				"id: TS.acceptance.001\n" +
				"kind: acceptance\n" +
				"title: Canonical fields\n" +
				"statement_type: evidence\n" +
				"claim_layer: object\n" +
				"owner: human\n" +
				"status: active\n" +
				"valid_until: 2026-07-24\n" +
				"terms:\n" +
				"  - WorkCommission\n" +
				"  - HarnessableProject\n" +
				"depends_on:\n" +
				"  - TS.environment-change.001\n" +
				"target_refs:\n" +
				"  - ES.test-strategy.001\n" +
				"evidence_required:\n" +
				"  - kind: review\n" +
				"    description: Human confirms acceptance still matches target-system intent.\n" +
				"  - Runtime evidence links to this section.\n" +
				"```\n",
		},
		{
			Path: ".haft/specs/term-map.md",
			Kind: "term-map",
			Content: "```yaml term-map\n" +
				"entries:\n" +
				"  - term: WorkCommission\n" +
				"    domain: enabling\n" +
				"    definition: Human-authorized bounded permission to execute a DecisionRecord.\n" +
				"    not:\n" +
				"      - DecisionRecord\n" +
				"    aliases:\n" +
				"      - commission\n" +
				"    owners:\n" +
				"      - haft\n" +
				"```\n",
		},
	})

	if len(specSet.Findings) != 0 {
		t.Fatalf("findings = %+v, want none", specSet.Findings)
	}
	if len(specSet.Documents) != 2 {
		t.Fatalf("documents = %#v, want two documents", specSet.Documents)
	}
	if len(specSet.Sections) != 1 {
		t.Fatalf("sections = %#v, want one section", specSet.Sections)
	}

	section := specSet.Sections[0]
	if section.ID != "TS.acceptance.001" {
		t.Fatalf("id = %q, want TS.acceptance.001", section.ID)
	}
	if section.Spec != "target-system" {
		t.Fatalf("spec = %q, want target-system", section.Spec)
	}
	if section.Kind != "acceptance" {
		t.Fatalf("kind = %q, want acceptance", section.Kind)
	}
	if section.StatementType != "evidence" {
		t.Fatalf("statement_type = %q, want evidence", section.StatementType)
	}
	if section.ClaimLayer != "object" {
		t.Fatalf("claim_layer = %q, want object", section.ClaimLayer)
	}
	if section.Owner != "human" {
		t.Fatalf("owner = %q, want human", section.Owner)
	}
	if section.Status != "active" {
		t.Fatalf("status = %q, want active", section.Status)
	}
	if section.ValidUntil != "2026-07-24" {
		t.Fatalf("valid_until = %q, want 2026-07-24", section.ValidUntil)
	}
	if !sameStrings(section.Terms, []string{"HarnessableProject", "WorkCommission"}) {
		t.Fatalf("terms = %#v, want canonical terms", section.Terms)
	}
	if !sameStrings(section.DependsOn, []string{"TS.environment-change.001"}) {
		t.Fatalf("depends_on = %#v, want dependency ref", section.DependsOn)
	}
	if !sameStrings(section.TargetRefs, []string{"ES.test-strategy.001"}) {
		t.Fatalf("target_refs = %#v, want target ref", section.TargetRefs)
	}
	if len(section.EvidenceRequired) != 2 {
		t.Fatalf("evidence_required = %#v, want two entries", section.EvidenceRequired)
	}
	if section.EvidenceRequired[0].Kind != "review" {
		t.Fatalf("first evidence kind = %q, want review", section.EvidenceRequired[0].Kind)
	}
	if section.EvidenceRequired[1].Description != "Runtime evidence links to this section." {
		t.Fatalf("second evidence description = %q, want string requirement", section.EvidenceRequired[1].Description)
	}

	if len(specSet.TermMapEntries) != 1 {
		t.Fatalf("term map entries = %#v, want one entry", specSet.TermMapEntries)
	}
	term := specSet.TermMapEntries[0]
	if term.Term != "WorkCommission" {
		t.Fatalf("term = %q, want WorkCommission", term.Term)
	}
	if term.Domain != "enabling" {
		t.Fatalf("domain = %q, want enabling", term.Domain)
	}
	if term.Definition == "" {
		t.Fatalf("definition = %q, want non-empty definition", term.Definition)
	}
	if !sameStrings(term.Not, []string{"DecisionRecord"}) {
		t.Fatalf("not = %#v, want DecisionRecord", term.Not)
	}
	if !sameStrings(term.Aliases, []string{"commission"}) {
		t.Fatalf("aliases = %#v, want commission", term.Aliases)
	}
	if !sameStrings(term.Owners, []string{"haft"}) {
		t.Fatalf("owners = %#v, want haft", term.Owners)
	}
}

func TestProjectSpecificationSetDistinguishesSectionLifecycleStates(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	specSet := ProjectSpecificationSetFromDocuments([]SpecDocumentInput{{
		Path: ".haft/specs/target-system.md",
		Kind: "target-system",
		Content: lifecycleSpecSection("TS.lifecycle.draft", "draft", "carrier", "null", true) +
			lifecycleSpecSection("TS.lifecycle.active", "active", "object", "2026-07-24", true) +
			lifecycleSpecSection("TS.lifecycle.deprecated", "deprecated", "object", "2026-07-24", true) +
			lifecycleSpecSection("TS.lifecycle.superseded", "superseded", "object", "2026-07-24", true) +
			lifecycleSpecSection("TS.lifecycle.stale", "active", "object", "2026-01-01", true) +
			lifecycleSpecSection("TS.lifecycle.malformed", "active", "object", "2026-07-24", false),
	}})

	sections := specSectionsByID(specSet.Sections)
	assertSpecSectionState(t, sections, "TS.lifecycle.draft", now, SpecSectionStateDraft)
	assertSpecSectionState(t, sections, "TS.lifecycle.active", now, SpecSectionStateActive)
	assertSpecSectionState(t, sections, "TS.lifecycle.deprecated", now, SpecSectionStateDeprecated)
	assertSpecSectionState(t, sections, "TS.lifecycle.superseded", now, SpecSectionStateSuperseded)
	assertSpecSectionState(t, sections, "TS.lifecycle.stale", now, SpecSectionStateStale)
	assertSpecSectionState(t, sections, "TS.lifecycle.malformed", now, SpecSectionStateMalformed)

	if !hasSpecCheckFinding(SpecCheckReportFromSpecificationSet(specSet), "spec_section_missing_field") {
		t.Fatalf("findings = %+v, want malformed section finding", specSet.Findings)
	}
}

func TestCheckSpecDocumentsReportsDraftCarrierReadinessGaps(t *testing.T) {
	report := CheckSpecDocuments([]SpecDocumentInput{
		{
			Path:    ".haft/specs/target-system.md",
			Kind:    "target-system",
			Content: targetSystemSpecCarrierContent(),
		},
		{
			Path:    ".haft/specs/enabling-system.md",
			Kind:    "enabling-system",
			Content: enablingSystemSpecCarrierContent(),
		},
		{
			Path:    ".haft/specs/term-map.md",
			Kind:    "term-map",
			Content: termMapSpecCarrierContent(),
		},
	})

	if !hasSpecCheckFinding(report, "spec_carrier_no_active_sections") {
		t.Fatalf("findings = %+v, want no-active-section readiness finding", report.Findings)
	}
	if !hasSpecCheckFinding(report, "term_map_missing_term") {
		t.Fatalf("findings = %+v, want empty term-map finding", report.Findings)
	}
	assertAllSpecCheckFindingsHaveNextAction(t, report)
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
				"terms: HarnessableProject\n" +
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
	assertSpecCheckFindingAt(t, report, "spec_section_invalid_terms", "$.terms")
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

func TestCheckSpecificationSetReportsIgnoredSpecCarriers(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".haft", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeSpecCheckFixture(t, filepath.Join(root, ".gitignore"), ".haft\n")
	writeSpecCheckFixture(t, filepath.Join(specDir, "target-system.md"), validSpecSectionCarrier("TS.use.001", "environment-change", "active"))
	writeSpecCheckFixture(t, filepath.Join(specDir, "enabling-system.md"), validSpecSectionCarrier("ES.creator.001", "creator-role", "active"))
	writeSpecCheckFixture(t, filepath.Join(specDir, "term-map.md"), validTermMapCarrier())

	report, err := CheckSpecificationSet(root)
	if err != nil {
		t.Fatalf("CheckSpecificationSet: %v", err)
	}
	if !hasSpecCheckFinding(report, "spec_carriers_gitignored") {
		t.Fatalf("report findings = %+v, want ignored carrier finding", report.Findings)
	}
	assertAllSpecCheckFindingsHaveNextAction(t, report)
}

func lifecycleSpecSection(
	id string,
	status string,
	claimLayer string,
	validUntil string,
	includeOwner bool,
) string {
	lines := []string{
		"## " + id,
		"",
		"```yaml spec-section",
		"id: " + id,
		"kind: acceptance",
		"statement_type: definition",
		"claim_layer: " + claimLayer,
	}
	if includeOwner {
		lines = append(lines, "owner: human")
	}
	lines = append(lines,
		"status: "+status,
		"valid_until: "+validUntil,
		"terms: []",
		"depends_on: []",
		"target_refs: []",
		"evidence_required: []",
		"```",
		"",
	)

	return strings.Join(lines, "\n")
}

func specSectionsByID(sections []SpecSection) map[string]SpecSection {
	byID := make(map[string]SpecSection, len(sections))

	for _, section := range sections {
		byID[section.ID] = section
	}

	return byID
}

func assertSpecSectionState(
	t *testing.T,
	sections map[string]SpecSection,
	id string,
	now time.Time,
	want SpecSectionState,
) {
	t.Helper()

	section, ok := sections[id]
	if !ok {
		t.Fatalf("sections = %#v, want %s", sections, id)
	}
	if got := section.LifecycleState(now); got != want {
		t.Fatalf("%s state = %q, want %q; section = %#v", id, got, want, section)
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

func assertAllSpecCheckFindingsHaveNextAction(t *testing.T, report SpecCheckReport) {
	t.Helper()

	for _, finding := range report.Findings {
		if strings.TrimSpace(finding.NextAction) == "" {
			t.Fatalf("finding missing next_action: %+v", finding)
		}
	}
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
