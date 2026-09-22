package fpf

import (
	"strings"
	"testing"
)

func dpfPublicationFixture() PublicationInput {
	return PublicationInput{
		Descriptor: PublicationDescriptor{ID: "dpf-syse", Kind: "engineering_dpf", Namespace: "SYSE", Path: "Engineering DPF Suite/SYSE.md", Root: "Systems Engineering"},
		Document: SourceDocument{Path: "data/FPF/Engineering DPF Suite/SYSE.md", SourceRevision: strings.Repeat("a", 40), Markdown: []byte(strings.Join([]string{
			"# Systems Engineering",
			"# Table of Contents",
			"| ID | Title |",
			"| [SYSE.1 - Select a system](#syse1) | question |",
			"| [SYSE.25 - Improve delivery](#syse25) | question |",
			"# Methods",
			"## SYSE.1 - Select a system",
			"### SYSE.1:1 - Problem",
			"Which system?",
			"### SYSE.1:4 - Solution",
			"Keep the full solution.",
			"```markdown",
			"## SYSE.999 - This is a fenced example",
			"```",
			"## SYSE.25 - Improve delivery",
			"### SYSE.25:4 - Solution",
			"Keep all delivery guidance.",
		}, "\n"))},
	}
}

func TestDPFPublicationPreservesBodyNamespaceAndDocumentIdentity(t *testing.T) {
	input := dpfPublicationFixture()
	units, err := buildPublicationNavigation(input)
	if err != nil {
		t.Fatal(err)
	}
	bodies := map[string]SourceUnit{}
	for _, unit := range units {
		if unit.Role == SourceUnitRolePatternBody {
			bodies[unit.PatternID] = unit
		}
	}
	if len(bodies) != 2 {
		t.Fatalf("bodies = %v", bodies)
	}
	unit := bodies["SYSE.1"]
	if unit.Provenance.PublicationKind != "engineering_dpf" || unit.Provenance.Namespace != "SYSE" {
		t.Fatalf("provenance = %+v", unit.Provenance)
	}
	digest := digestSourceDocument(input.Document)
	if unit.Provenance.DocumentDigest != digest.String() {
		t.Fatal("document digest differs from exact bytes")
	}
	if unit.Provenance.StartLine != 7 || unit.Provenance.EndLine != 14 {
		t.Fatalf("range = %+v", unit.Provenance)
	}
	if !strings.Contains(unit.Body, "Keep the full solution.") || !strings.Contains(unit.Body, "SYSE.999") {
		t.Fatal("full body lost text or fenced example")
	}
	if strings.Contains(unit.Body, "Improve delivery") {
		t.Fatal("body broadened into next pattern")
	}
}

func TestDPFPublicationRejectsDuplicateAndForeignIdentities(t *testing.T) {
	cases := []struct {
		name   string
		suffix string
	}{
		{name: "duplicate root", suffix: "\n# Systems Engineering\n"},
		{name: "foreign pattern", suffix: "\n## ME.1 - Foreign pattern\nbody"},
		{name: "unlisted pattern", suffix: "\n## SYSE.99 - No ToC row\nbody"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := dpfPublicationFixture()
			input.Document.Markdown = append(input.Document.Markdown, []byte(test.suffix)...)
			if _, err := buildPublicationNavigation(input); err == nil {
				t.Fatal("malformed publication accepted")
			}
		})
	}
}

func TestUsageGuideRetainsCompleteNavigationWithoutDefiningDPFPatterns(t *testing.T) {
	publications := SourceAccessPublications()
	descriptor := publications[0]
	markdown := "# Using FPF and its DPF Suites\n\n## Choose what to read\nUse the source.\n\n```markdown\n## SYSE.24 - Example only\n```\n\n## Search and read\nRead the full body.\n"
	input := PublicationInput{
		Descriptor: descriptor,
		Document: SourceDocument{
			Path: "data/FPF/USING-FPF.md", SourceRevision: strings.Repeat("b", 40), Markdown: []byte(markdown),
		},
	}
	units, err := buildPublicationNavigation(input)
	if err != nil {
		t.Fatal(err)
	}
	whole := units[0]
	if whole.SourceID != "fpf-usage-guide" || whole.Role != SourceUnitRolePublication || whole.Provenance.PublicationKind != "usage_guide" {
		t.Fatalf("guide identity = %+v", whole)
	}
	if whole.Body != strings.TrimSpace(markdown) {
		t.Fatal("guide publication lost source text")
	}
	for _, unit := range units {
		if unit.Role == SourceUnitRolePatternBody || unit.Role == SourceUnitRolePatternSection {
			t.Fatalf("guide example became a pattern: %s", unit.UnitID)
		}
	}
}
