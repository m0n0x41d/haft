package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectReadinessClassifiesMissingProject(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")

	facts, err := InspectReadiness(root)
	if err != nil {
		t.Fatalf("InspectReadiness: %v", err)
	}

	if facts.Status != ReadinessMissing {
		t.Fatalf("status = %q, want %q", facts.Status, ReadinessMissing)
	}
}

func TestInspectReadinessClassifiesProjectWithoutHaftAsNeedsInit(t *testing.T) {
	root := t.TempDir()

	facts, err := InspectReadiness(root)
	if err != nil {
		t.Fatalf("InspectReadiness: %v", err)
	}

	if facts.Status != ReadinessNeedsInit {
		t.Fatalf("status = %q, want %q", facts.Status, ReadinessNeedsInit)
	}
	if !facts.Exists || facts.HasHaft {
		t.Fatalf("facts = %+v, want exists=true has_haft=false", facts)
	}
}

func TestInspectReadinessClassifiesInitializedProjectAsNeedsOnboard(t *testing.T) {
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(haftDir, "project.yaml"), []byte("id: qnt_test\nname: test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	facts, err := InspectReadiness(root)
	if err != nil {
		t.Fatalf("InspectReadiness: %v", err)
	}

	if facts.Status != ReadinessNeedsOnboard {
		t.Fatalf("status = %q, want %q", facts.Status, ReadinessNeedsOnboard)
	}
	if !facts.Exists || !facts.HasHaft || facts.HasSpecs {
		t.Fatalf("facts = %+v, want exists=true has_haft=true has_specs=false", facts)
	}
}

func TestInspectReadinessClassifiesEmptyOnboardingCarriersAsNeedsOnboard(t *testing.T) {
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	specDir := filepath.Join(haftDir, "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(haftDir, "project.yaml"), "id: qnt_test\nname: test\n")
	writeFixture(t, filepath.Join(haftDir, "workflow.md"), "# Workflow\n\n## Defaults\n\n```yaml\nmode: standard\n```\n")
	writeFixture(t, filepath.Join(specDir, "target-system.md"), "# Target System Spec\n")
	writeFixture(t, filepath.Join(specDir, "enabling-system.md"), "# Enabling System Spec\n")
	writeFixture(t, filepath.Join(specDir, "term-map.md"), "# Term Map\n")

	facts, err := InspectReadiness(root)
	if err != nil {
		t.Fatalf("InspectReadiness: %v", err)
	}

	if facts.Status != ReadinessNeedsOnboard {
		t.Fatalf("status = %q, want %q", facts.Status, ReadinessNeedsOnboard)
	}
	if !facts.Exists || !facts.HasHaft || facts.HasSpecs {
		t.Fatalf("facts = %+v, want exists=true has_haft=true has_specs=false", facts)
	}
}

func TestInspectReadinessClassifiesDraftSpecCarriersAsNeedsOnboard(t *testing.T) {
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(haftDir, "project.yaml"), "id: qnt_test\nname: test\n")
	writeFixture(t, filepath.Join(haftDir, "workflow.md"), "# Workflow\n\n## Defaults\n\n```yaml\nmode: standard\n```\n")
	if err := EnsureSpecCarriers(haftDir); err != nil {
		t.Fatalf("EnsureSpecCarriers: %v", err)
	}

	facts, err := InspectReadiness(root)
	if err != nil {
		t.Fatalf("InspectReadiness: %v", err)
	}

	if facts.Status != ReadinessNeedsOnboard {
		t.Fatalf("status = %q, want %q", facts.Status, ReadinessNeedsOnboard)
	}
	if !facts.Exists || !facts.HasHaft || facts.HasSpecs {
		t.Fatalf("facts = %+v, want exists=true has_haft=true has_specs=false", facts)
	}
}

func TestInspectReadinessClassifiesMinimumSpecSetAsReady(t *testing.T) {
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	specDir := filepath.Join(haftDir, "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(haftDir, "project.yaml"), "id: qnt_test\nname: test\n")
	writeFixture(t, filepath.Join(haftDir, "workflow.md"), "# Workflow\n\n## Defaults\n\n```yaml\nmode: standard\n```\n")
	writeFixture(t, filepath.Join(specDir, "target-system.md"), readinessSpecSection("TS.use.001", "environment-change"))
	writeFixture(t, filepath.Join(specDir, "enabling-system.md"), readinessSpecSection("ES.creator.001", "creator-role"))
	writeFixture(t, filepath.Join(specDir, "term-map.md"), "```yaml\nterm: HarnessableProject\ndomain: enabling\ndefinition: A project with active specs.\n```\n")

	facts, err := InspectReadiness(root)
	if err != nil {
		t.Fatalf("InspectReadiness: %v", err)
	}

	if facts.Status != ReadinessReady {
		t.Fatalf("status = %q, want %q", facts.Status, ReadinessReady)
	}
	if !facts.Exists || !facts.HasHaft || !facts.HasSpecs {
		t.Fatalf("facts = %+v, want exists=true has_haft=true has_specs=true", facts)
	}
}

func TestInspectReadinessClassifiesMalformedActiveSpecAsNeedsOnboard(t *testing.T) {
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	specDir := filepath.Join(haftDir, "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(haftDir, "project.yaml"), "id: qnt_test\nname: test\n")
	writeFixture(t, filepath.Join(haftDir, "workflow.md"), "# Workflow\n\n## Defaults\n\n```yaml\nmode: standard\n```\n")
	writeFixture(t, filepath.Join(specDir, "target-system.md"), malformedActiveReadinessSpecSection("TS.use.001", "environment-change"))
	writeFixture(t, filepath.Join(specDir, "enabling-system.md"), readinessSpecSection("ES.creator.001", "creator-role"))
	writeFixture(t, filepath.Join(specDir, "term-map.md"), "```yaml\nterm: HarnessableProject\ndomain: enabling\ndefinition: A project with active specs.\n```\n")

	facts, err := InspectReadiness(root)
	if err != nil {
		t.Fatalf("InspectReadiness: %v", err)
	}

	if facts.Status != ReadinessNeedsOnboard {
		t.Fatalf("status = %q, want %q", facts.Status, ReadinessNeedsOnboard)
	}
	if !facts.Exists || !facts.HasHaft || facts.HasSpecs {
		t.Fatalf("facts = %+v, want exists=true has_haft=true has_specs=false", facts)
	}
}

func TestInspectReadinessClassifiesMissingTermMapAsNeedsOnboard(t *testing.T) {
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	specDir := filepath.Join(haftDir, "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(haftDir, "project.yaml"), "id: qnt_test\nname: test\n")
	writeFixture(t, filepath.Join(haftDir, "workflow.md"), "# Workflow\n\n## Defaults\n\n```yaml\nmode: standard\n```\n")
	writeFixture(t, filepath.Join(specDir, "target-system.md"), readinessSpecSection("TS.use.001", "environment-change"))
	writeFixture(t, filepath.Join(specDir, "enabling-system.md"), readinessSpecSection("ES.creator.001", "creator-role"))

	facts, err := InspectReadiness(root)
	if err != nil {
		t.Fatalf("InspectReadiness: %v", err)
	}

	if facts.Status != ReadinessNeedsOnboard {
		t.Fatalf("status = %q, want %q", facts.Status, ReadinessNeedsOnboard)
	}
	if !facts.Exists || !facts.HasHaft || facts.HasSpecs {
		t.Fatalf("facts = %+v, want exists=true has_haft=true has_specs=false", facts)
	}
}

func writeFixture(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readinessSpecSection(id string, kind string) string {
	return "## " + id + "\n\n" +
		"```yaml spec-section\n" +
		"id: " + id + "\n" +
		"kind: " + kind + "\n" +
		"statement_type: definition\n" +
		"claim_layer: object\n" +
		"owner: human\n" +
		"status: active\n" +
		"```\n"
}

func malformedActiveReadinessSpecSection(id string, kind string) string {
	return "## " + id + "\n\n" +
		"```yaml spec-section\n" +
		"id: " + id + "\n" +
		"kind: " + kind + "\n" +
		"statement_type: definition\n" +
		"claim_layer: object\n" +
		"owner: human\n" +
		"status: active\n" +
		"terms: [\n" +
		"```\n"
}
